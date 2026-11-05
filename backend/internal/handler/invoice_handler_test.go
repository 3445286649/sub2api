//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type invoiceHandlerRepo struct {
	service.InvoiceRepository
	rows    []service.InvoiceOrderRow
	request *service.InvoiceRequest
}

func (r *invoiceHandlerRepo) ListOrdersForUser(context.Context, int64, []string) ([]service.InvoiceOrderRow, error) {
	return r.rows, nil
}

func (r *invoiceHandlerRepo) CreateRequest(_ context.Context, req *service.InvoiceRequest, _ []int64, buildLines func([]service.InvoiceOrderRow) ([]service.InvoiceOrderLine, error)) error {
	if _, err := buildLines(r.rows); err != nil {
		return err
	}
	req.ID = 1001
	req.CreatedAt = time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	r.request = req
	return nil
}

func (r *invoiceHandlerRepo) GetRequest(_ context.Context, id int64) (*service.InvoiceRequest, error) {
	if r.request == nil || r.request.ID != id {
		return nil, service.ErrInvoiceRequestNotFound
	}
	return r.request, nil
}

func (r *invoiceHandlerRepo) ListRequests(_ context.Context, params pagination.PaginationParams, _ service.InvoiceRequestListFilters) ([]service.InvoiceRequest, *pagination.PaginationResult, error) {
	return []service.InvoiceRequest{*r.request}, &pagination.PaginationResult{Total: 1, Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func newInvoiceTestRouter(repo service.InvoiceRepository, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewInvoiceHandler(service.NewInvoiceService(repo, nil, nil, nil, nil))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/invoice/orders", h.ListOrders)
	router.GET("/invoice/requests", h.List)
	router.POST("/invoice/requests", h.Create)
	router.GET("/invoice/requests/:id", h.GetByID)
	return router
}

func invoiceHandlerRequest(t *testing.T, router *gin.Engine, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded), rec.Body.String())
	return rec.Code, decoded
}

// 字段名要和 frontend/src/types/invoice.ts 对齐，前端按这些名字取值。
func TestInvoiceOrdersResponseMatchesFrontendContract(t *testing.T) {
	repo := &invoiceHandlerRepo{rows: []service.InvoiceOrderRow{{
		ID: 1, OutTradeNo: "sub2_a", RechargeCode: "PAY-1-1", OrderType: "balance", PaymentType: "alipay",
		Status: payment.OrderStatusCompleted, Amount: 110, PayAmount: 100, BonusAmount: 10, PaidAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	}}}

	code, body := invoiceHandlerRequest(t, newInvoiceTestRouter(repo, 7), http.MethodGet, "/invoice/orders", "")

	require.Equal(t, http.StatusOK, code)
	data := body["data"].(map[string]any)
	summary := data["summary"].(map[string]any)
	require.EqualValues(t, 100, summary["total_invoiceable_amount"])
	require.EqualValues(t, 1, summary["invoiceable_count"])
	order := data["items"].([]any)[0].(map[string]any)
	require.Equal(t, "sub2_a", order["out_trade_no"])
	require.Equal(t, "PAY-1-1", order["recharge_code"])
	require.EqualValues(t, 100, order["invoiceable_amount"])
	require.EqualValues(t, 10, order["bonus_amount"])
	require.Equal(t, false, order["already_invoiced"])
	require.Equal(t, false, order["excluded"])
	require.Equal(t, "2026-11-01T00:00:00Z", order["paid_at"])
}

func TestInvoiceOrdersEmptyListIsAnArrayNotNull(t *testing.T) {
	_, body := invoiceHandlerRequest(t, newInvoiceTestRouter(&invoiceHandlerRepo{}, 7), http.MethodGet, "/invoice/orders", "")
	require.Equal(t, []any{}, body["data"].(map[string]any)["items"])
}

func TestInvoiceCreateListAndOwnership(t *testing.T) {
	repo := &invoiceHandlerRepo{rows: []service.InvoiceOrderRow{{ID: 1, Status: payment.OrderStatusCompleted, Amount: 100, PayAmount: 100}}}
	router := newInvoiceTestRouter(repo, 7)
	payload := `{"invoice_type":"enterprise","order_ids":[1],"receiver_email":"a@b.cn","enterprise_name":"某某公司","enterprise_tax_number":"91330106MA2KXXXX7Q"}`

	code, body := invoiceHandlerRequest(t, router, http.MethodPost, "/invoice/requests", payload)
	require.Equal(t, http.StatusCreated, code)
	created := body["data"].(map[string]any)
	require.EqualValues(t, 1001, created["id"])
	require.Equal(t, "某某公司", created["title"])
	require.EqualValues(t, 100, created["total_amount"])
	require.EqualValues(t, 1, created["order_count"])
	require.Equal(t, "pending", created["status"])
	require.NotContains(t, created, "admin_notes")
	require.NotContains(t, created, "user_email")

	code, body = invoiceHandlerRequest(t, router, http.MethodGet, "/invoice/requests?page=1&page_size=10", "")
	require.Equal(t, http.StatusOK, code)
	page := body["data"].(map[string]any)
	require.EqualValues(t, 1, page["total"])
	require.EqualValues(t, 1, page["pages"])
	require.Len(t, page["items"], 1)

	repo.request.UserID = 7
	code, _ = invoiceHandlerRequest(t, router, http.MethodGet, "/invoice/requests/1001", "")
	require.Equal(t, http.StatusOK, code)

	code, body = invoiceHandlerRequest(t, newInvoiceTestRouter(repo, 8), http.MethodGet, "/invoice/requests/1001", "")
	require.Equal(t, http.StatusNotFound, code, "other users cannot read the request")
	require.Equal(t, "INVOICE_REQUEST_NOT_FOUND", body["reason"])
}

func TestInvoiceCreateRefusesAnExcludedOrderWithAStableReason(t *testing.T) {
	repo := &invoiceHandlerRepo{rows: []service.InvoiceOrderRow{{ID: 1, Status: payment.OrderStatusCompleted, Amount: 100, PayAmount: 100, Excluded: true}}}
	payload := `{"invoice_type":"individual","order_ids":[1],"receiver_email":"a@b.cn","individual_name":"张三"}`

	code, body := invoiceHandlerRequest(t, newInvoiceTestRouter(repo, 7), http.MethodPost, "/invoice/requests", payload)

	require.Equal(t, http.StatusConflict, code)
	require.Equal(t, "ORDER_NOT_INVOICEABLE", body["reason"], "the frontend maps this reason to a localized message")
}
