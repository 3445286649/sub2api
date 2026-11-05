//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminInvoiceFileRepo struct {
	service.InvoiceRepository
	request *service.InvoiceRequest
}

func (r *adminInvoiceFileRepo) GetRequest(_ context.Context, id int64) (*service.InvoiceRequest, error) {
	if r.request == nil || r.request.ID != id {
		return nil, service.ErrInvoiceRequestNotFound
	}
	copied := *r.request
	return &copied, nil
}

func (r *adminInvoiceFileRepo) SetFile(_ context.Context, id int64, meta service.InvoiceFileMeta) (*string, error) {
	if r.request == nil || r.request.ID != id {
		return nil, service.ErrInvoiceRequestNotFound
	}
	if r.request.Status == service.InvoiceStatusRejected {
		return nil, service.ErrInvoiceFileStatus
	}
	previous := r.request.InvoiceFileKey
	name, key, size := meta.Name, meta.Key, meta.Size
	r.request.InvoiceFileName, r.request.InvoiceFileKey, r.request.InvoiceFileSize = &name, &key, &size
	return previous, nil
}

type adminInvoiceFileStore struct{ data map[string][]byte }

func (s *adminInvoiceFileStore) Save(_ context.Context, data []byte) (string, error) {
	key := "0123456789abcdef0123456789abcdef.pdf"
	s.data[key] = append([]byte(nil), data...)
	return key, nil
}

func (s *adminInvoiceFileStore) Read(_ context.Context, key string) ([]byte, error) {
	data, ok := s.data[key]
	if !ok {
		return nil, service.ErrInvoiceFileNotFound
	}
	return data, nil
}

func (s *adminInvoiceFileStore) Delete(_ context.Context, key string) error {
	delete(s.data, key)
	return nil
}

func newAdminInvoiceFileRouter(status string) (*gin.Engine, *adminInvoiceFileRepo, *adminInvoiceFileStore) {
	gin.SetMode(gin.TestMode)
	repo := &adminInvoiceFileRepo{request: &service.InvoiceRequest{ID: 5, UserID: 7, Status: status}}
	files := &adminInvoiceFileStore{data: map[string][]byte{}}
	h := NewInvoiceHandler(service.NewInvoiceService(repo, nil, nil, nil, files))
	router := gin.New()
	router.POST("/invoice/requests/:id/file", h.UploadFile)
	router.GET("/invoice/requests/:id/file", h.DownloadFile)
	return router, repo, files
}

func adminInvoiceUpload(t *testing.T, router *gin.Engine, field, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/invoice/requests/5/file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAdminInvoiceFileUploadStoresThePDFAndReportsMetadata(t *testing.T) {
	router, repo, files := newAdminInvoiceFileRouter(service.InvoiceStatusProcessing)
	pdf := []byte("%PDF-1.7 invoice")

	rec := adminInvoiceUpload(t, router, "file", "发票 26332.pdf", pdf)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, true, body.Data["has_file"])
	require.Equal(t, "发票 26332.pdf", body.Data["invoice_file_name"])
	require.EqualValues(t, len(pdf), body.Data["invoice_file_size"])
	require.NotContains(t, rec.Body.String(), "invoice_file_key")
	require.NotNil(t, repo.request.InvoiceFileKey)
	require.Equal(t, pdf, files.data[*repo.request.InvoiceFileKey])
}

func TestAdminInvoiceFileUploadRefusesBadUploads(t *testing.T) {
	oversized := append([]byte("%PDF-"), make([]byte, service.InvoiceFileMaxBytes)...)
	cases := []struct {
		name     string
		field    string
		filename string
		content  []byte
		want     int
	}{
		{"html renamed to pdf", "file", "invoice.pdf", []byte("<html><script>alert(1)</script></html>"), http.StatusBadRequest},
		{"empty file", "file", "invoice.pdf", nil, http.StatusBadRequest},
		{"wrong form field", "upload", "invoice.pdf", []byte("%PDF-1.7"), http.StatusBadRequest},
		{"over the size limit", "file", "invoice.pdf", oversized, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, repo, files := newAdminInvoiceFileRouter(service.InvoiceStatusPending)

			rec := adminInvoiceUpload(t, router, tc.field, tc.filename, tc.content)

			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			require.Nil(t, repo.request.InvoiceFileKey)
			require.Empty(t, files.data)
		})
	}
}

func TestAdminInvoiceFileUploadIsRefusedForRejectedRequests(t *testing.T) {
	router, repo, files := newAdminInvoiceFileRouter(service.InvoiceStatusRejected)

	rec := adminInvoiceUpload(t, router, "file", "invoice.pdf", []byte("%PDF-1.7"))

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Nil(t, repo.request.InvoiceFileKey)
	require.Empty(t, files.data, "the rejected upload must not leave a file behind")
}

func TestAdminInvoiceFileDownloadWorksBeforeTheInvoiceIsIssued(t *testing.T) {
	router, _, _ := newAdminInvoiceFileRouter(service.InvoiceStatusPending)
	pdf := []byte("%PDF-1.7 draft")
	require.Equal(t, http.StatusOK, adminInvoiceUpload(t, router, "file", "draft.pdf", pdf).Code)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invoice/requests/5/file", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, pdf, rec.Body.Bytes())
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))

	missing, _, _ := newAdminInvoiceFileRouter(service.InvoiceStatusPending)
	rec = httptest.NewRecorder()
	missing.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invoice/requests/5/file", nil))
	require.Equal(t, http.StatusNotFound, rec.Code, "no file uploaded yet")
}
