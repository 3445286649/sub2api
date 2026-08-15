package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DailyCheckinHandler struct {
	service *service.DailyCheckinService
}

func NewDailyCheckinHandler(service *service.DailyCheckinService) *DailyCheckinHandler {
	return &DailyCheckinHandler{service: service}
}

func (h *DailyCheckinHandler) Status(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	result, err := h.service.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DailyCheckinHandler) Claim(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	result, err := h.service.Claim(c.Request.Context(), subject.UserID, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DailyCheckinHandler) History(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	page, pageSize := response.ParsePagination(c)
	result, err := h.service.History(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
