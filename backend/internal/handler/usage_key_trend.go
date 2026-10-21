package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/gin-gonic/gin"
)

// parseKeyUsageTrendOptions reads the optional comparison period and bucket settings.
func parseKeyUsageTrendOptions(c *gin.Context, start, end time.Time) (usagestats.APIKeyUsageTrendOptions, bool) {
	var opts usagestats.APIKeyUsageTrendOptions
	rawPrevStart, rawPrevEnd := c.Query("prev_start_time"), c.Query("prev_end_time")
	if rawPrevStart != "" || rawPrevEnd != "" {
		prevStart, startErr := time.Parse(time.RFC3339, rawPrevStart)
		prevEnd, endErr := time.Parse(time.RFC3339, rawPrevEnd)
		if startErr != nil || endErr != nil || !prevStart.Before(prevEnd) || prevEnd.After(start) || prevEnd.Sub(prevStart) > 366*24*time.Hour {
			response.BadRequest(c, "Invalid previous time range")
			return opts, false
		}
		opts.PrevStartTime, opts.PrevEndTime = &prevStart, &prevEnd
	}
	switch opts.Granularity = c.Query("granularity"); opts.Granularity {
	case "", usagestats.KeyUsageGranularityDay:
	case usagestats.KeyUsageGranularityHour:
		if end.Sub(start) > 48*time.Hour {
			response.BadRequest(c, "Hourly granularity supports at most 2 days")
			return opts, false
		}
	default:
		response.BadRequest(c, "Invalid granularity")
		return opts, false
	}
	opts.Location = time.UTC
	if raw := c.Query("timezone"); raw != "" {
		loc, err := time.LoadLocation(raw)
		if err != nil || raw == "Local" {
			response.BadRequest(c, "Invalid timezone")
			return opts, false
		}
		opts.Location = loc
	}
	return opts, true
}
