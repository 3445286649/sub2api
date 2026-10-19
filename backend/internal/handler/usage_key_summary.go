package handler

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// APIKeySummary aggregates only the authenticated user's keys, including deleted keys with history.
func (h *UsageHandler) APIKeySummary(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	start, end, ok := parseExactUsageRange(c)
	if !ok {
		return
	}
	q := usagestats.APIKeySummaryQuery{
		UserID: subject.UserID, StartTime: start, EndTime: end,
		Search:    strings.TrimSpace(c.Query("search")),
		SortBy:    c.DefaultQuery("sort_by", "actual_cost"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
		Page:      1, PageSize: 20,
	}
	if utf8.RuneCountInString(q.Search) > 100 {
		response.BadRequest(c, "Search is too long")
		return
	}
	if q.SortBy != "actual_cost" && q.SortBy != "total_tokens" && q.SortBy != "requests" {
		response.BadRequest(c, "Invalid sort_by")
		return
	}
	if q.SortOrder != "asc" && q.SortOrder != "desc" {
		response.BadRequest(c, "Invalid sort_order")
		return
	}
	for name, target := range map[string]*int64{"api_key_id": &q.APIKeyID, "group_id": &q.GroupID} {
		if raw := c.Query(name); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || v <= 0 {
				response.BadRequest(c, "Invalid "+name)
				return
			}
			*target = v
		}
	}
	for name, target := range map[string]*int{"page": &q.Page, "page_size": &q.PageSize} {
		if raw := c.Query(name); raw != "" {
			v, err := strconv.Atoi(raw)
			max := 100
			if name == "page" {
				max = 100000
			}
			if err != nil || v < 1 || v > max {
				response.BadRequest(c, "Invalid "+name)
				return
			}
			*target = v
		}
	}
	if raw := c.Query("only_used"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			response.BadRequest(c, "Invalid only_used")
			return
		}
		q.OnlyUsed = v
	}
	result, err := h.usageService.GetUserAPIKeySummary(c.Request.Context(), q)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseExactUsageRange(c *gin.Context) (time.Time, time.Time, bool) {
	start, startErr := time.Parse(time.RFC3339, c.Query("start_time"))
	end, endErr := time.Parse(time.RFC3339, c.Query("end_time"))
	if startErr != nil || endErr != nil || !start.Before(end) || end.Sub(start) > 366*24*time.Hour {
		response.BadRequest(c, "Invalid time range; use RFC3339 timestamps, maximum 366 days")
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}
