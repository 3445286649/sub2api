//go:build integration

package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newInvoiceFileFixture(t *testing.T) (*service.InvoiceService, service.InvoiceRepository, *localInvoiceFileStore, *service.InvoiceRequest) {
	t.Helper()
	repo := NewInvoiceRepository(integrationDB)
	store := newLocalInvoiceFileStore(t.TempDir())
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: invoiceTestUnique("invoice") + "@example.com"})
	svc := service.NewInvoiceService(repo, nil, nil, nil, store)
	order := mustCreateInvoiceOrder(t, user.ID, user.Email, "COMPLETED", 100)
	created, err := svc.CreateRequest(context.Background(), enterpriseInvoiceInput(user.ID, order.ID))
	require.NoError(t, err)
	return svc, repo, store, created
}

func invoiceFileMeta(name, key string, size int64) service.InvoiceFileMeta {
	return service.InvoiceFileMeta{Name: name, Key: key, Size: size, UpdatedAt: time.Now()}
}

func TestInvoiceRepositorySetFileReturnsThePreviousKey(t *testing.T) {
	ctx := context.Background()
	_, repo, _, req := newInvoiceFileFixture(t)

	previous, err := repo.SetFile(ctx, req.ID, invoiceFileMeta("a.pdf", "key-a.pdf", 10))
	require.NoError(t, err)
	require.Nil(t, previous, "the first upload replaces nothing")

	previous, err = repo.SetFile(ctx, req.ID, invoiceFileMeta("b.pdf", "key-b.pdf", 20))
	require.NoError(t, err)
	require.NotNil(t, previous)
	require.Equal(t, "key-a.pdf", *previous)

	got, err := repo.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, "b.pdf", *got.InvoiceFileName)
	require.Equal(t, "key-b.pdf", *got.InvoiceFileKey)
	require.EqualValues(t, 20, *got.InvoiceFileSize)

	_, err = repo.SetFile(ctx, 999999999, invoiceFileMeta("c.pdf", "key-c.pdf", 1))
	require.ErrorIs(t, err, service.ErrInvoiceRequestNotFound)
}

func TestInvoiceRepositorySetFileRefusesRejectedRequests(t *testing.T) {
	ctx := context.Background()
	svc, repo, _, req := newInvoiceFileFixture(t)
	_, err := repo.SetFile(ctx, req.ID, invoiceFileMeta("a.pdf", "key-a.pdf", 10))
	require.NoError(t, err)

	_, err = svc.UpdateStatus(ctx, req.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusRejected, RejectReason: "抬头有误"})
	require.NoError(t, err)

	_, err = repo.SetFile(ctx, req.ID, invoiceFileMeta("b.pdf", "key-b.pdf", 20))
	require.ErrorIs(t, err, service.ErrInvoiceFileStatus)
	got, err := repo.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, "key-a.pdf", *got.InvoiceFileKey, "a refused upload leaves the stored file untouched")
}

// 并发上传时，每个旧 key 只能被一次上传拿到：所有返回的旧 key 加上最后留下的 key，
// 恰好等于全部上传过的 key，既不重复也不遗漏，否则会漏删磁盘上的文件。
func TestInvoiceRepositorySetFileConcurrentUploadsLoseNoKey(t *testing.T) {
	ctx := context.Background()
	_, repo, _, req := newInvoiceFileFixture(t)
	const uploads = 20

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make([]string, 0, uploads)
	var firstErr error
	for i := 0; i < uploads; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			previous, err := repo.SetFile(ctx, req.ID, invoiceFileMeta(fmt.Sprintf("%d.pdf", i), fmt.Sprintf("key-%02d.pdf", i), int64(i+1)))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				firstErr = err
				return
			}
			if previous != nil {
				seen = append(seen, *previous)
			}
		}(i)
	}
	wg.Wait()
	require.NoError(t, firstErr)

	got, err := repo.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	seen = append(seen, *got.InvoiceFileKey)

	want := make([]string, 0, uploads)
	for i := 0; i < uploads; i++ {
		want = append(want, fmt.Sprintf("key-%02d.pdf", i))
	}
	require.ElementsMatch(t, want, seen)
}

func TestInvoiceFileUploadReplaceAndDownloadAgainstRealStorage(t *testing.T) {
	ctx := context.Background()
	svc, _, store, req := newInvoiceFileFixture(t)
	firstPDF := []byte("%PDF-1.7 first")
	secondPDF := []byte("%PDF-1.4 second")
	storedFiles := func() []string {
		matches, err := filepath.Glob(filepath.Join(store.directory, "*.pdf"))
		require.NoError(t, err)
		return matches
	}

	uploaded, err := svc.UploadFile(ctx, req.ID, "first.pdf", firstPDF)
	require.NoError(t, err)
	require.Equal(t, "first.pdf", *uploaded.InvoiceFileName)
	require.Len(t, storedFiles(), 1)

	// 开票前用户看不到草稿文件。
	_, _, err = svc.DownloadFileForUser(ctx, req.UserID, req.ID)
	require.ErrorIs(t, err, service.ErrInvoiceFileNotFound)

	replaced, err := svc.UploadFile(ctx, req.ID, "second.pdf", secondPDF)
	require.NoError(t, err)
	require.NotEqual(t, *uploaded.InvoiceFileKey, *replaced.InvoiceFileKey)
	require.Len(t, storedFiles(), 1, "the replaced file is removed from disk")

	_, err = svc.UpdateStatus(ctx, req.ID, service.UpdateInvoiceStatusInput{Status: service.InvoiceStatusCompleted, InvoiceNumber: "26332"})
	require.NoError(t, err, "an uploaded file is enough to complete without an external link")

	name, data, err := svc.DownloadFileForUser(ctx, req.UserID, req.ID)
	require.NoError(t, err)
	require.Equal(t, "second.pdf", name)
	require.Equal(t, secondPDF, data)

	_, _, err = svc.DownloadFileForUser(ctx, req.UserID+1, req.ID)
	require.ErrorIs(t, err, service.ErrInvoiceRequestNotFound, "other users cannot download it")
}
