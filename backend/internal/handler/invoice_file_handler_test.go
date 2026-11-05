//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testInvoiceFileKey = "0123456789abcdef0123456789abcdef.pdf"

type invoiceFileRepoStub struct {
	service.InvoiceRepository
	request *service.InvoiceRequest
}

func (r *invoiceFileRepoStub) GetRequest(_ context.Context, id int64) (*service.InvoiceRequest, error) {
	if r.request == nil || r.request.ID != id {
		return nil, service.ErrInvoiceRequestNotFound
	}
	copied := *r.request
	return &copied, nil
}

type invoiceFileStoreStub struct{ data map[string][]byte }

func (s *invoiceFileStoreStub) Save(context.Context, []byte) (string, error) { return "", nil }

func (s *invoiceFileStoreStub) Read(_ context.Context, key string) ([]byte, error) {
	data, ok := s.data[key]
	if !ok {
		return nil, service.ErrInvoiceFileNotFound
	}
	return data, nil
}

func (s *invoiceFileStoreStub) Delete(context.Context, string) error { return nil }

func newInvoiceFileRouter(repo service.InvoiceRepository, files service.InvoiceFileStore, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewInvoiceHandler(service.NewInvoiceService(repo, nil, nil, nil, files))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/invoice/requests/:id", h.GetByID)
	router.GET("/invoice/requests/:id/file", h.DownloadFile)
	return router
}

func invoiceFileGet(router *gin.Engine, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func completedInvoiceWithFile() *service.InvoiceRequest {
	name, key, size := "发票.pdf", testInvoiceFileKey, int64(16)
	return &service.InvoiceRequest{
		ID: 5, UserID: 7, Status: service.InvoiceStatusCompleted,
		InvoiceFileName: &name, InvoiceFileKey: &key, InvoiceFileSize: &size,
	}
}

func TestInvoiceFileDownloadSendsAPrivateAttachment(t *testing.T) {
	pdf := []byte("%PDF-1.7 invoice")
	router := newInvoiceFileRouter(
		&invoiceFileRepoStub{request: completedInvoiceWithFile()},
		&invoiceFileStoreStub{data: map[string][]byte{testInvoiceFileKey: pdf}}, 7)

	rec := invoiceFileGet(router, "/invoice/requests/5/file")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, pdf, rec.Body.Bytes())
	require.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
	disposition, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	require.Equal(t, "attachment", disposition)
	require.Equal(t, "发票.pdf", params["filename"])
}

func TestInvoiceFileDownloadIsRefusedForOtherUsersAndUnissuedInvoices(t *testing.T) {
	files := &invoiceFileStoreStub{data: map[string][]byte{testInvoiceFileKey: []byte("%PDF-1.7 invoice")}}

	other := newInvoiceFileRouter(&invoiceFileRepoStub{request: completedInvoiceWithFile()}, files, 8)
	require.Equal(t, http.StatusNotFound, invoiceFileGet(other, "/invoice/requests/5/file").Code)

	processing := completedInvoiceWithFile()
	processing.Status = service.InvoiceStatusProcessing
	mine := newInvoiceFileRouter(&invoiceFileRepoStub{request: processing}, files, 7)
	rec := invoiceFileGet(mine, "/invoice/requests/5/file")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.NotContains(t, rec.Body.String(), "%PDF")

	require.Equal(t, http.StatusNotFound, invoiceFileGet(mine, "/invoice/requests/99/file").Code)
}

func TestInvoiceFileDownloadWhenTheStoredFileIsGone(t *testing.T) {
	router := newInvoiceFileRouter(
		&invoiceFileRepoStub{request: completedInvoiceWithFile()},
		&invoiceFileStoreStub{data: map[string][]byte{}}, 7)
	require.Equal(t, http.StatusNotFound, invoiceFileGet(router, "/invoice/requests/5/file").Code)
}

func TestInvoiceFileDownloadWithoutStorageConfigured(t *testing.T) {
	router := newInvoiceFileRouter(&invoiceFileRepoStub{request: completedInvoiceWithFile()}, nil, 7)
	require.Equal(t, http.StatusServiceUnavailable, invoiceFileGet(router, "/invoice/requests/5/file").Code)
}

func TestInvoiceResponseShowsFileMetadataButNeverTheStorageKey(t *testing.T) {
	files := &invoiceFileStoreStub{data: map[string][]byte{testInvoiceFileKey: []byte("%PDF-1.7 invoice")}}

	router := newInvoiceFileRouter(&invoiceFileRepoStub{request: completedInvoiceWithFile()}, files, 7)
	rec := invoiceFileGet(router, "/invoice/requests/5")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), testInvoiceFileKey)
	require.NotContains(t, rec.Body.String(), "invoice_file_key")
	var completed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &completed))
	require.Equal(t, true, completed.Data["has_file"])
	require.Equal(t, "发票.pdf", completed.Data["invoice_file_name"])
	require.EqualValues(t, 16, completed.Data["invoice_file_size"])

	// 开票前管理员上传的草稿，用户看不到。
	draft := completedInvoiceWithFile()
	draft.Status = service.InvoiceStatusProcessing
	router = newInvoiceFileRouter(&invoiceFileRepoStub{request: draft}, files, 7)
	rec = invoiceFileGet(router, "/invoice/requests/5")
	require.Equal(t, http.StatusOK, rec.Code)
	var processing struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &processing))
	require.Equal(t, false, processing.Data["has_file"])
	require.Nil(t, processing.Data["invoice_file_name"])
}
