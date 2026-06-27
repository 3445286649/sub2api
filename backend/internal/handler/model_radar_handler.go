package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ModelRadarHandler struct {
	service *service.ModelRadarService
}

func NewModelRadarHandler(service *service.ModelRadarService) *ModelRadarHandler {
	return &ModelRadarHandler{service: service}
}

func (h *ModelRadarHandler) Current(c *gin.Context) {
	if h == nil || h.service == nil {
		response.ErrorFrom(c, service.ErrModelRadarDisabled)
		return
	}
	current, err := h.service.GetPublicCurrent(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, current)
}
