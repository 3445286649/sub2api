package service

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *ChannelMonitorService) ListDisplayCategories(ctx context.Context) ([]string, error) {
	return s.repo.ListDisplayCategories(ctx)
}

func validateMonitorDisplayCategory(value string) error {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 50 || strings.ContainsFunc(value, unicode.IsControl) {
		return infraerrors.BadRequest("INVALID_MONITOR_DISPLAY_CATEGORY", "Display category must be at most 50 characters without control characters")
	}
	return nil
}

func validateMonitorDisplayOrder(value int) error {
	if value < 0 || value > 999999 {
		return infraerrors.BadRequest("INVALID_MONITOR_DISPLAY_ORDER", "Display order must be an integer between 0 and 999999")
	}
	return nil
}
