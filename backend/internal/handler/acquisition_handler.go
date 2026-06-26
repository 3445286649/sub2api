package handler

import (
	"fmt"
	"net/url"

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
	if summary != nil && summary.AffCode != "" && summary.InviteLink == "" {
		summary.InviteLink = buildAcquisitionInviteLink(c, summary.AffCode)
	}
	response.Success(c, summary)
}

func buildAcquisitionInviteLink(c *gin.Context, affCode string) string {
	if affCode == "" || c == nil || c.Request == nil {
		return ""
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	if forwarded := c.GetHeader("X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	if host == "" {
		return fmt.Sprintf("/register?aff=%s", url.QueryEscape(affCode))
	}
	return fmt.Sprintf("%s://%s/register?aff=%s", scheme, host, url.QueryEscape(affCode))
}
