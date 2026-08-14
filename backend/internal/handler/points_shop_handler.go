package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PointsShopHandler struct {
	service *service.PointsShopService
}

func NewPointsShopHandler(service *service.PointsShopService) *PointsShopHandler {
	return &PointsShopHandler{service: service}
}

func (h *PointsShopHandler) Summary(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	account, err := h.service.GetAccount(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"account": account, "config": cfg})
}

func (h *PointsShopHandler) Products(c *gin.Context) {
	products, err := h.service.ListProducts(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, products)
}

func (h *PointsShopHandler) Redeem(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	productID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || productID <= 0 {
		response.BadRequest(c, "invalid product id")
		return
	}
	var req struct {
		IdempotencyKey string `json:"idempotency_key" binding:"required,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	order, err := h.service.Redeem(c.Request.Context(), subject.UserID, productID, req.IdempotencyKey)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, order)
}

func (h *PointsShopHandler) Ledger(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.ListUserLedger(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PointsShopHandler) Orders(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.ListOrders(c.Request.Context(), subject.UserID, false, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
