//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeInvoiceFiles struct {
	data    map[string][]byte
	next    int
	readErr error
}

func newFakeInvoiceFiles() *fakeInvoiceFiles { return &fakeInvoiceFiles{data: map[string][]byte{}} }

func (f *fakeInvoiceFiles) Save(_ context.Context, data []byte) (string, error) {
	f.next++
	key := fmt.Sprintf("%032x.pdf", f.next)
	f.data[key] = append([]byte(nil), data...)
	return key, nil
}

func (f *fakeInvoiceFiles) Read(_ context.Context, key string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	data, ok := f.data[key]
	if !ok {
		return nil, ErrInvoiceFileNotFound
	}
	return data, nil
}

func (f *fakeInvoiceFiles) Delete(_ context.Context, key string) error {
	delete(f.data, key)
	return nil
}

// SetFile 模拟仓储：已驳回的申请不能再上传，其余状态覆盖元数据并返回旧 key。
func (f *fakeInvoiceRepo) SetFile(_ context.Context, id int64, meta InvoiceFileMeta) (*string, error) {
	if f.request == nil || f.request.ID != id {
		return nil, ErrInvoiceRequestNotFound
	}
	if f.request.Status == InvoiceStatusRejected {
		return nil, ErrInvoiceFileStatus
	}
	previous := f.request.InvoiceFileKey
	name, key, size := meta.Name, meta.Key, meta.Size
	f.request.InvoiceFileName, f.request.InvoiceFileKey, f.request.InvoiceFileSize = &name, &key, &size
	return previous, nil
}

type fakeAttachmentMailer struct {
	fakeInvoiceMailer
	attachment *EmailAttachment
	plainSent  bool
}

func (f *fakeAttachmentMailer) SendEmail(ctx context.Context, to, subject, body string) error {
	f.plainSent = true
	return f.fakeInvoiceMailer.SendEmail(ctx, to, subject, body)
}

func (f *fakeAttachmentMailer) SendEmailWithAttachment(_ context.Context, to, _, _ string, attachment *EmailAttachment) error {
	f.to = to
	f.attachment = attachment
	return nil
}

var (
	testInvoicePDF  = []byte("%PDF-1.7\n%first\n")
	testInvoicePDF2 = []byte("%PDF-1.4\n%second\n")
)

func newInvoiceFileTestService(status string) (*InvoiceService, *fakeInvoiceRepo, *fakeInvoiceFiles, *fakeAttachmentMailer) {
	repo := &fakeInvoiceRepo{updateOK: true, request: &InvoiceRequest{
		ID: 5, UserID: 7, Title: "某某公司", ReceiverEmail: "finance@example.com", Status: status,
	}}
	files := newFakeInvoiceFiles()
	mailer := &fakeAttachmentMailer{}
	svc := &InvoiceService{repo: repo, email: mailer, site: fakeInvoiceSite{}, encryptor: fakeInvoiceEncryptor{}, files: files}
	return svc, repo, files, mailer
}

func TestValidateInvoicePDF(t *testing.T) {
	oversized := append([]byte("%PDF-"), make([]byte, InvoiceFileMaxBytes)...)
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"a real pdf header is accepted", testInvoicePDF, nil},
		{"empty upload", nil, ErrInvoiceFileRequired},
		{"html renamed to pdf", []byte("<html><script>alert(1)</script></html>"), ErrInvoiceFileNotPDF},
		{"png renamed to pdf", []byte("\x89PNG\r\n\x1a\n"), ErrInvoiceFileNotPDF},
		{"pdf marker not at the start", []byte(" %PDF-1.7"), ErrInvoiceFileNotPDF},
		{"over the size limit", oversized, ErrInvoiceFileTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, validateInvoicePDF(tc.data))
		})
	}
}

func TestSanitizeInvoiceFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"invoice.pdf", "invoice.pdf"},
		{"../../etc/passwd", "passwd.pdf"},
		{`C:\x\发票.pdf`, "发票.pdf"},
		{"发票 26332.PDF", "发票 26332.pdf"},
		{"report", "report.pdf"},
		{`a"b<c>.pdf`, "abc.pdf"},
		{"line\r\nbreak.pdf", "linebreak.pdf"},
		{"", "invoice-5.pdf"},
		{"..", "invoice-5.pdf"},
		{".pdf", "invoice-5.pdf"},
		{strings.Repeat("a", 300) + ".pdf", strings.Repeat("a", 100) + ".pdf"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, sanitizeInvoiceFileName(tc.in, 5), tc.in)
	}
}

func TestUploadFileReplacesTheOldFileAfterTheDatabaseUpdate(t *testing.T) {
	svc, _, files, _ := newInvoiceFileTestService(InvoiceStatusPending)
	ctx := context.Background()

	first, err := svc.UploadFile(ctx, 5, "first.pdf", testInvoicePDF)
	require.NoError(t, err)
	require.NotNil(t, first.InvoiceFileKey)
	firstKey := *first.InvoiceFileKey
	require.Equal(t, "first.pdf", *first.InvoiceFileName)
	require.Equal(t, int64(len(testInvoicePDF)), *first.InvoiceFileSize)

	second, err := svc.UploadFile(ctx, 5, "second.pdf", testInvoicePDF2)
	require.NoError(t, err)
	require.NotEqual(t, firstKey, *second.InvoiceFileKey)
	require.NotContains(t, files.data, firstKey)
	require.Len(t, files.data, 1)
	require.Equal(t, testInvoicePDF2, files.data[*second.InvoiceFileKey])
}

func TestUploadFileRefusesBadInputWithoutTouchingStorage(t *testing.T) {
	svc, _, files, _ := newInvoiceFileTestService(InvoiceStatusPending)
	ctx := context.Background()

	_, err := svc.UploadFile(ctx, 5, "a.pdf", []byte("<html></html>"))
	require.Equal(t, ErrInvoiceFileNotPDF, err)
	_, err = svc.UploadFile(ctx, 99, "a.pdf", testInvoicePDF)
	require.Equal(t, ErrInvoiceRequestNotFound, err)
	require.Empty(t, files.data)

	svc.files = nil
	_, err = svc.UploadFile(ctx, 5, "a.pdf", testInvoicePDF)
	require.Equal(t, ErrInvoiceFileUnavailable, err)
}

func TestUploadToARejectedRequestCleansUpTheNewFileAndKeepsTheOldOne(t *testing.T) {
	svc, repo, files, _ := newInvoiceFileTestService(InvoiceStatusPending)
	ctx := context.Background()
	first, err := svc.UploadFile(ctx, 5, "first.pdf", testInvoicePDF)
	require.NoError(t, err)

	repo.request.Status = InvoiceStatusRejected
	_, err = svc.UploadFile(ctx, 5, "second.pdf", testInvoicePDF2)

	require.Equal(t, ErrInvoiceFileStatus, err)
	require.Len(t, files.data, 1)
	require.Contains(t, files.data, *first.InvoiceFileKey)
}

func TestUserCanOnlyDownloadTheirOwnCompletedInvoice(t *testing.T) {
	svc, repo, _, _ := newInvoiceFileTestService(InvoiceStatusPending)
	ctx := context.Background()
	_, err := svc.UploadFile(ctx, 5, "发票.pdf", testInvoicePDF)
	require.NoError(t, err)

	// 开票前上传的文件对用户不可见。
	_, _, err = svc.DownloadFileForUser(ctx, 7, 5)
	require.Equal(t, ErrInvoiceFileNotFound, err)

	repo.request.Status = InvoiceStatusCompleted
	name, data, err := svc.DownloadFileForUser(ctx, 7, 5)
	require.NoError(t, err)
	require.Equal(t, "发票.pdf", name)
	require.Equal(t, testInvoicePDF, data)

	// 别人的申请与不存在的申请表现一致。
	_, _, err = svc.DownloadFileForUser(ctx, 8, 5)
	require.Equal(t, ErrInvoiceRequestNotFound, err)
}

func TestUserDownloadWithoutAnUploadedFileIsNotFound(t *testing.T) {
	svc, _, _, _ := newInvoiceFileTestService(InvoiceStatusCompleted)
	_, _, err := svc.DownloadFileForUser(context.Background(), 7, 5)
	require.Equal(t, ErrInvoiceFileNotFound, err)
}

func TestAdminCanDownloadAtAnyStatus(t *testing.T) {
	svc, _, _, _ := newInvoiceFileTestService(InvoiceStatusProcessing)
	ctx := context.Background()
	_, err := svc.UploadFile(ctx, 5, "draft.pdf", testInvoicePDF)
	require.NoError(t, err)

	name, data, err := svc.DownloadFileForAdmin(ctx, 5)

	require.NoError(t, err)
	require.Equal(t, "draft.pdf", name)
	require.Equal(t, testInvoicePDF, data)
}

func TestCompletionAcceptsAnUploadedFileInsteadOfALink(t *testing.T) {
	svc, _, _, _ := newInvoiceFileTestService(InvoiceStatusProcessing)
	ctx := context.Background()
	input := UpdateInvoiceStatusInput{Status: InvoiceStatusCompleted, InvoiceNumber: "26332"}

	_, err := svc.UpdateStatus(ctx, 5, input)
	require.Equal(t, ErrInvoiceFileURLInvalid, err, "neither a link nor an upload")

	_, err = svc.UploadFile(ctx, 5, "invoice.pdf", testInvoicePDF)
	require.NoError(t, err)
	updated, err := svc.UpdateStatus(ctx, 5, input)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCompleted, updated.Status)

	input.InvoiceFileURL = "javascript:alert(1)"
	_, err = svc.UpdateStatus(ctx, 5, input)
	require.Equal(t, ErrInvoiceFileURLInvalid, err, "a bad link is still refused even when a file exists")
}

func TestCompletionEmailCarriesTheUploadedPDF(t *testing.T) {
	svc, repo, _, mailer := newInvoiceFileTestService(InvoiceStatusProcessing)
	ctx := context.Background()
	_, err := svc.UploadFile(ctx, 5, "invoice.pdf", testInvoicePDF)
	require.NoError(t, err)

	_, err = svc.UpdateStatus(ctx, 5, UpdateInvoiceStatusInput{Status: InvoiceStatusCompleted, InvoiceNumber: "26332", SendEmail: true})

	require.NoError(t, err)
	require.NotNil(t, mailer.attachment)
	require.Equal(t, "invoice.pdf", mailer.attachment.Filename)
	require.Equal(t, "application/pdf", mailer.attachment.ContentType)
	require.Equal(t, testInvoicePDF, mailer.attachment.Data)
	require.Equal(t, "finance@example.com", mailer.to)
	require.False(t, mailer.plainSent)
	require.NotNil(t, repo.emailedAt)
}

func TestCompletionEmailFallsBackToTextWhenTheFileCannotBeRead(t *testing.T) {
	svc, repo, files, mailer := newInvoiceFileTestService(InvoiceStatusProcessing)
	ctx := context.Background()
	_, err := svc.UploadFile(ctx, 5, "invoice.pdf", testInvoicePDF)
	require.NoError(t, err)
	files.readErr = errors.New("disk offline")

	_, err = svc.UpdateStatus(ctx, 5, UpdateInvoiceStatusInput{Status: InvoiceStatusCompleted, InvoiceNumber: "26332", SendEmail: true})

	require.NoError(t, err)
	require.Nil(t, mailer.attachment)
	require.True(t, mailer.plainSent)
	require.NotNil(t, repo.emailedAt)
}
