//go:build unit

package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func categoryHandlerRepo(categories ...service.InvoiceCategory) *invoiceHandlerRepo {
	return &invoiceHandlerRepo{
		rows:       []service.InvoiceOrderRow{{ID: 1, Status: payment.OrderStatusCompleted, Amount: 100, PayAmount: 100, PaidAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}},
		categories: categories,
	}
}

// 用户只看到启用的类目，且只有下拉框需要的三个字段。
func TestInvoiceCategoriesListOnlyEnabledOptionsWithDisplayFields(t *testing.T) {
	repo := categoryHandlerRepo(
		service.InvoiceCategory{ID: 1, Name: "技术服务费", Description: "软件技术服务", Enabled: true, SortOrder: 1, RequestCount: 4},
		service.InvoiceCategory{ID: 2, Name: "已停用类目", Enabled: false},
	)

	code, body := invoiceHandlerRequest(t, newInvoiceTestRouter(repo, 7), http.MethodGet, "/invoice/categories", "")

	require.Equal(t, http.StatusOK, code)
	items := body["data"].([]any)
	require.Len(t, items, 1)
	option := items[0].(map[string]any)
	require.EqualValues(t, 1, option["id"])
	require.Equal(t, "技术服务费", option["name"])
	require.Equal(t, "软件技术服务", option["description"])
	require.NotContains(t, option, "enabled")
	require.NotContains(t, option, "sort_order")
	require.NotContains(t, option, "request_count")
}

// 没配置任何类目时必须是空数组，前端据此隐藏下拉框。
func TestInvoiceCategoriesEmptyListIsAnArrayNotNull(t *testing.T) {
	code, body := invoiceHandlerRequest(t, newInvoiceTestRouter(categoryHandlerRepo(), 7), http.MethodGet, "/invoice/categories", "")

	require.Equal(t, http.StatusOK, code)
	require.Equal(t, []any{}, body["data"])
}

func TestInvoiceCreateWithCategoryReturnsTheSnapshotFields(t *testing.T) {
	repo := categoryHandlerRepo(
		service.InvoiceCategory{ID: 1, Name: "技术服务费", Enabled: true},
		service.InvoiceCategory{ID: 2, Name: "软件使用费", Enabled: true},
	)
	payload := `{"invoice_type":"individual","order_ids":[1],"receiver_email":"a@b.cn","individual_name":"张三","category_id":2}`

	code, body := invoiceHandlerRequest(t, newInvoiceTestRouter(repo, 7), http.MethodPost, "/invoice/requests", payload)

	require.Equal(t, http.StatusCreated, code)
	created := body["data"].(map[string]any)
	require.EqualValues(t, 2, created["category_id"])
	require.Equal(t, "软件使用费", created["category_name"])
}

func TestInvoiceCreateRefusesMissingOrDisabledCategoryWithStableReasons(t *testing.T) {
	repo := categoryHandlerRepo(
		service.InvoiceCategory{ID: 1, Name: "技术服务费", Enabled: true},
		service.InvoiceCategory{ID: 2, Name: "已停用类目", Enabled: false},
	)
	router := newInvoiceTestRouter(repo, 7)
	base := `{"invoice_type":"individual","order_ids":[1],"receiver_email":"a@b.cn","individual_name":"张三"%s}`

	code, body := invoiceHandlerRequest(t, router, http.MethodPost, "/invoice/requests", sprintfCategory(base, ""))
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, "INVOICE_CATEGORY_REQUIRED", body["reason"], "the frontend maps this reason to a localized message")

	code, body = invoiceHandlerRequest(t, router, http.MethodPost, "/invoice/requests", sprintfCategory(base, `,"category_id":2`))
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, "INVALID_INVOICE_CATEGORY", body["reason"])

	code, body = invoiceHandlerRequest(t, router, http.MethodPost, "/invoice/requests", sprintfCategory(base, `,"category_id":99`))
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, "INVALID_INVOICE_CATEGORY", body["reason"])
	require.Nil(t, repo.request, "nothing is stored when the category is refused")
}

// 没配置类目时旧流程不变：不传类目也能提交，返回里类目字段为 null。
func TestInvoiceCreateWithoutConfiguredCategoriesStillWorks(t *testing.T) {
	payload := `{"invoice_type":"individual","order_ids":[1],"receiver_email":"a@b.cn","individual_name":"张三"}`

	code, body := invoiceHandlerRequest(t, newInvoiceTestRouter(categoryHandlerRepo(), 7), http.MethodPost, "/invoice/requests", payload)

	require.Equal(t, http.StatusCreated, code)
	created := body["data"].(map[string]any)
	require.Contains(t, created, "category_id")
	require.Nil(t, created["category_id"])
	require.Nil(t, created["category_name"])
}

func sprintfCategory(format, extra string) string {
	out := make([]byte, 0, len(format)+len(extra))
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && format[i+1] == 's' {
			out = append(out, extra...)
			i++
			continue
		}
		out = append(out, format[i])
	}
	return string(out)
}
