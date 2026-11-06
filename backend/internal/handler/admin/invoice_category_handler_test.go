//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// adminCategoryRepo 只实现类目相关的方法，其余方法调用会 panic，说明测试越界了。
type adminCategoryRepo struct {
	service.InvoiceRepository
	items  []service.InvoiceCategory
	used   map[int64]bool
	nextID int64
}

func (r *adminCategoryRepo) index(id int64) int {
	for i := range r.items {
		if r.items[i].ID == id {
			return i
		}
	}
	return -1
}

func (r *adminCategoryRepo) nameTaken(name string, exceptID int64) bool {
	for _, item := range r.items {
		if item.Name == name && item.ID != exceptID {
			return true
		}
	}
	return false
}

func (r *adminCategoryRepo) ListCategories(_ context.Context, enabledOnly bool) ([]service.InvoiceCategory, error) {
	out := make([]service.InvoiceCategory, 0, len(r.items))
	for _, item := range r.items {
		if enabledOnly && !item.Enabled {
			continue
		}
		if r.used[item.ID] {
			item.RequestCount = 2
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *adminCategoryRepo) GetCategory(_ context.Context, id int64) (*service.InvoiceCategory, error) {
	i := r.index(id)
	if i < 0 {
		return nil, service.ErrInvoiceCategoryNotFound
	}
	item := r.items[i]
	return &item, nil
}

func (r *adminCategoryRepo) CreateCategory(_ context.Context, c *service.InvoiceCategory) error {
	if r.nameTaken(c.Name, 0) {
		return service.ErrInvoiceCategoryDuplicate
	}
	r.nextID++
	c.ID = r.nextID
	r.items = append(r.items, *c)
	return nil
}

func (r *adminCategoryRepo) UpdateCategory(_ context.Context, c *service.InvoiceCategory) error {
	i := r.index(c.ID)
	if i < 0 {
		return service.ErrInvoiceCategoryNotFound
	}
	if r.nameTaken(c.Name, c.ID) {
		return service.ErrInvoiceCategoryDuplicate
	}
	r.items[i] = *c
	return nil
}

func (r *adminCategoryRepo) DeleteCategory(_ context.Context, id int64) error {
	i := r.index(id)
	if i < 0 {
		return service.ErrInvoiceCategoryNotFound
	}
	if r.used[id] {
		return service.ErrInvoiceCategoryInUse
	}
	r.items = append(r.items[:i], r.items[i+1:]...)
	return nil
}

func (r *adminCategoryRepo) ReorderCategories(_ context.Context, ids []int64) error {
	ordered := make([]service.InvoiceCategory, 0, len(ids))
	for position, id := range ids {
		i := r.index(id)
		if i < 0 {
			return service.ErrInvoiceCategoryNotFound
		}
		item := r.items[i]
		item.SortOrder = position + 1
		ordered = append(ordered, item)
	}
	r.items = ordered
	return nil
}

func newAdminCategoryRouter(repo *adminCategoryRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewInvoiceHandler(service.NewInvoiceService(repo, nil, nil, nil, nil))
	router := gin.New()
	router.GET("/invoice/categories", h.ListCategories)
	router.POST("/invoice/categories", h.CreateCategory)
	router.PUT("/invoice/categories/order", h.ReorderCategories)
	router.PATCH("/invoice/categories/:id", h.UpdateCategory)
	router.DELETE("/invoice/categories/:id", h.DeleteCategory)
	return router
}

func adminCategoryCall(t *testing.T, router *gin.Engine, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded), rec.Body.String())
	return rec.Code, decoded
}

func TestAdminInvoiceCategoryCreateUpdateListAndDelete(t *testing.T) {
	router := newAdminCategoryRouter(&adminCategoryRepo{})

	code, body := adminCategoryCall(t, router, http.MethodPost, "/invoice/categories", `{"name":" 技术服务费 ","description":"软件技术服务"}`)
	require.Equal(t, http.StatusCreated, code)
	created := body["data"].(map[string]any)
	require.EqualValues(t, 1, created["id"])
	require.Equal(t, "技术服务费", created["name"], "names are trimmed")
	require.Equal(t, true, created["enabled"], "new categories are enabled by default")
	require.Contains(t, created, "sort_order")
	require.Contains(t, created, "request_count")

	// 只改启停：名称和说明保持不变。
	code, body = adminCategoryCall(t, router, http.MethodPatch, "/invoice/categories/1", `{"enabled":false}`)
	require.Equal(t, http.StatusOK, code)
	updated := body["data"].(map[string]any)
	require.Equal(t, false, updated["enabled"])
	require.Equal(t, "技术服务费", updated["name"])
	require.Equal(t, "软件技术服务", updated["description"])

	// 管理端列表包含已停用的类目。
	code, body = adminCategoryCall(t, router, http.MethodGet, "/invoice/categories", "")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, body["data"].([]any), 1)

	code, _ = adminCategoryCall(t, router, http.MethodDelete, "/invoice/categories/1", "")
	require.Equal(t, http.StatusOK, code)
	code, body = adminCategoryCall(t, router, http.MethodGet, "/invoice/categories", "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, []any{}, body["data"])
}

func TestAdminInvoiceCategoryRefusesBadInputWithStableReasons(t *testing.T) {
	repo := &adminCategoryRepo{
		items:  []service.InvoiceCategory{{ID: 1, Name: "技术服务费", Enabled: true}, {ID: 2, Name: "软件使用费", Enabled: true}},
		used:   map[int64]bool{2: true},
		nextID: 2,
	}
	router := newAdminCategoryRouter(repo)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		reason string
	}{
		{"create without a name", http.MethodPost, "/invoice/categories", `{}`, http.StatusBadRequest, "INVOICE_CATEGORY_NAME_REQUIRED"},
		{"create with a blank name", http.MethodPost, "/invoice/categories", `{"name":"   "}`, http.StatusBadRequest, "INVOICE_CATEGORY_NAME_REQUIRED"},
		{"create with a duplicate name", http.MethodPost, "/invoice/categories", `{"name":"技术服务费"}`, http.StatusConflict, "INVOICE_CATEGORY_DUPLICATE"},
		{"rename onto an existing name", http.MethodPatch, "/invoice/categories/1", `{"name":"软件使用费"}`, http.StatusConflict, "INVOICE_CATEGORY_DUPLICATE"},
		{"update a missing category", http.MethodPatch, "/invoice/categories/99", `{"enabled":false}`, http.StatusNotFound, "INVOICE_CATEGORY_NOT_FOUND"},
		{"delete a missing category", http.MethodDelete, "/invoice/categories/99", "", http.StatusNotFound, "INVOICE_CATEGORY_NOT_FOUND"},
		{"delete a category used by requests", http.MethodDelete, "/invoice/categories/2", "", http.StatusConflict, "INVOICE_CATEGORY_IN_USE"},
		{"reorder with duplicate ids", http.MethodPut, "/invoice/categories/order", `{"ids":[1,1]}`, http.StatusBadRequest, "INVOICE_CATEGORY_REORDER_INVALID"},
		{"reorder with an empty list", http.MethodPut, "/invoice/categories/order", `{"ids":[]}`, http.StatusBadRequest, "INVOICE_CATEGORY_REORDER_INVALID"},
		{"reorder with an unknown id", http.MethodPut, "/invoice/categories/order", `{"ids":[1,99]}`, http.StatusNotFound, "INVOICE_CATEGORY_NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := adminCategoryCall(t, router, tc.method, tc.path, tc.body)
			require.Equal(t, tc.status, code, body)
			require.Equal(t, tc.reason, body["reason"])
		})
	}

	// 非法路径参数在进入 service 之前就被拒绝。
	code, _ := adminCategoryCall(t, router, http.MethodPatch, "/invoice/categories/abc", `{"enabled":false}`)
	require.Equal(t, http.StatusBadRequest, code)

	require.Len(t, repo.items, 2, "refused requests change nothing")
}

func TestAdminInvoiceCategoryListShowsHowOftenEachIsUsed(t *testing.T) {
	repo := &adminCategoryRepo{
		items: []service.InvoiceCategory{{ID: 1, Name: "技术服务费", Enabled: true}, {ID: 2, Name: "软件使用费", Enabled: false}},
		used:  map[int64]bool{2: true},
	}

	_, body := adminCategoryCall(t, newAdminCategoryRouter(repo), http.MethodGet, "/invoice/categories", "")

	items := body["data"].([]any)
	require.Len(t, items, 2)
	require.EqualValues(t, 0, items[0].(map[string]any)["request_count"])
	require.EqualValues(t, 2, items[1].(map[string]any)["request_count"])
}

func TestAdminInvoiceCategoryReorderReturnsTheNewOrder(t *testing.T) {
	repo := &adminCategoryRepo{items: []service.InvoiceCategory{
		{ID: 1, Name: "甲", Enabled: true}, {ID: 2, Name: "乙", Enabled: true}, {ID: 3, Name: "丙", Enabled: true},
	}}

	code, body := adminCategoryCall(t, newAdminCategoryRouter(repo), http.MethodPut, "/invoice/categories/order", `{"ids":[3,1,2]}`)

	require.Equal(t, http.StatusOK, code)
	items := body["data"].([]any)
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.(map[string]any)["name"].(string))
	}
	require.Equal(t, []string{"丙", "甲", "乙"}, names)
	require.EqualValues(t, 1, items[0].(map[string]any)["sort_order"])
}

func TestParseInvoiceCategoryFilterIgnoresInvalidValues(t *testing.T) {
	for raw, want := range map[string]int64{"": 0, "abc": 0, "-4": 0, "0": 0, "12abc": 0, " 5 ": 5, "7": 7} {
		require.Equal(t, want, parseInvoiceCategoryFilter(raw), "input %q", raw)
	}
}
