package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PointsShopHandler struct {
	service *service.PointsShopService
}

func NewPointsShopHandler(service *service.PointsShopService) *PointsShopHandler {
	return &PointsShopHandler{service: service}
}

func (h *PointsShopHandler) GetConfig(c *gin.Context) {
	result, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PointsShopHandler) UpdateConfig(c *gin.Context) {
	var req service.PointsConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	result, err := h.service.UpdateConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PointsShopHandler) Products(c *gin.Context) {
	result, err := h.service.ListProducts(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PointsShopHandler) CreateProduct(c *gin.Context) {
	var req service.PointsProductInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	result, err := h.service.CreateProduct(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, result)
}

func (h *PointsShopHandler) UpdateProduct(c *gin.Context) {
	id, ok := pointsShopID(c)
	if !ok {
		return
	}
	var req service.PointsProductInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	result, err := h.service.UpdateProduct(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PointsShopHandler) DeleteProduct(c *gin.Context) {
	id, ok := pointsShopID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteProduct(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func (h *PointsShopHandler) Orders(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.ListOrders(c.Request.Context(), 0, true, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func pointsShopID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid product id")
		return 0, false
	}
	return id, true
}
