package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// ListCategories 返回已启用的开票类目。一个都没配置时返回空数组，前端据此不显示类目选择。
func (h *InvoiceHandler) ListCategories(c *gin.Context) {
	items, err := h.invoiceService.ListCategoriesForUser(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.InvoiceCategoryOptionsFromService(items))
}
