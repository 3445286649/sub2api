package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// 指针字段为 nil 表示不修改，所以启停和排序可以单独 PATCH。
type invoiceCategoryRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sort_order"`
}

type reorderInvoiceCategoriesRequest struct {
	IDs []int64 `json:"ids" binding:"required"`
}

func (r invoiceCategoryRequest) toInput() service.InvoiceCategoryInput {
	return service.InvoiceCategoryInput{
		Name:        r.Name,
		Description: r.Description,
		Enabled:     r.Enabled,
		SortOrder:   r.SortOrder,
	}
}

// 非法或缺省的 category_id 都按不筛选处理。
func parseInvoiceCategoryFilter(raw string) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func (h *InvoiceHandler) ListCategories(c *gin.Context) {
	items, err := h.invoiceService.ListCategoriesForAdmin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminInvoiceCategoriesFromService(items))
}

func (h *InvoiceHandler) CreateCategory(c *gin.Context) {
	var req invoiceCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	created, err := h.invoiceService.CreateCategory(c.Request.Context(), req.toInput())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.AdminInvoiceCategoryFromService(created))
}

func (h *InvoiceHandler) UpdateCategory(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid category ID")
	if !ok {
		return
	}
	var req invoiceCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	updated, err := h.invoiceService.UpdateCategory(c.Request.Context(), id, req.toInput())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminInvoiceCategoryFromService(updated))
}

func (h *InvoiceHandler) DeleteCategory(c *gin.Context) {
	id, ok := parsePositiveID(c, "id", "Invalid category ID")
	if !ok {
		return
	}
	if err := h.invoiceService.DeleteCategory(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

// ReorderCategories 按传入的 ID 顺序重写排序，返回排序后的完整列表。
func (h *InvoiceHandler) ReorderCategories(c *gin.Context) {
	var req reorderInvoiceCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.invoiceService.ReorderCategories(c.Request.Context(), req.IDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items, err := h.invoiceService.ListCategoriesForAdmin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminInvoiceCategoriesFromService(items))
}
