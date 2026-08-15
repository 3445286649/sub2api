package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DailyCheckinHandler struct {
	service *service.DailyCheckinService
}

func NewDailyCheckinHandler(service *service.DailyCheckinService) *DailyCheckinHandler {
	return &DailyCheckinHandler{service: service}
}

func (h *DailyCheckinHandler) GetConfig(c *gin.Context) {
	result, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DailyCheckinHandler) UpdateConfig(c *gin.Context) {
	var req service.DailyCheckinConfig
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

func (h *DailyCheckinHandler) Stats(c *gin.Context) {
	result, err := h.service.Stats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DailyCheckinHandler) Records(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.AdminRecords(c.Request.Context(), page, pageSize, c.Query("date"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
