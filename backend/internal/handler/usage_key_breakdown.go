package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// APIKeyBreakdown returns historical group/model usage for the caller's own key.
func (h *UsageHandler) APIKeyBreakdown(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || keyID <= 0 {
		response.BadRequest(c, "Invalid api_key_id")
		return
	}
	start, end, ok := parseExactUsageRange(c)
	if !ok {
		return
	}
	q := usagestats.APIKeyBreakdownQuery{
		UserID: subject.UserID, APIKeyID: keyID, StartTime: start, EndTime: end,
	}
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		groupID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || groupID <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		q.GroupID = groupID
	}
	result, err := h.usageService.GetUserAPIKeyBreakdown(c.Request.Context(), q)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
