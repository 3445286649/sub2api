package service

import (
	"context"
	"log/slog"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// EnsureSupportTicketsEnabled enforces the station-internal support ticket module switch.
// The switch is opt-out: missing or unreadable settings keep the legacy/default-enabled behavior.
func (s *SettingService) EnsureSupportTicketsEnabled(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	vals, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeySupportTicketsEnabled})
	if err != nil {
		slog.Warn("failed to get support_tickets_enabled setting, defaulting to true", "error", err)
		return nil
	}
	if isFalseSettingValue(vals[SettingKeySupportTicketsEnabled]) {
		return infraerrors.Forbidden("SUPPORT_TICKETS_DISABLED", "support tickets are disabled")
	}
	return nil
}
