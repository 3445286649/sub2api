package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AcquisitionHandler struct {
	acquisitionService *service.AcquisitionService
}

func NewAcquisitionHandler(acquisitionService *service.AcquisitionService) *AcquisitionHandler {
	return &AcquisitionHandler{acquisitionService: acquisitionService}
}

// GetCurrent returns the current user's active acquisition campaign dashboard.
// GET /api/v1/acquisition/current
func (h *AcquisitionHandler) GetCurrent(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.acquisitionService == nil {
		response.ErrorFrom(c, service.ErrAcquisitionDisabled)
		return
	}
	summary, err := h.acquisitionService.GetCurrentUserSummary(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}
