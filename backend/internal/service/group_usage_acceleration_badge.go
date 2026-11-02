package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func normalizeUsageAccelerationBadge(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "fast", "ultrafast", "free_fast", "billed_fast", "billed_ultrafast":
		return value, nil
	default:
		return "", infraerrors.BadRequest("INVALID_USAGE_ACCELERATION_BADGE", "invalid usage acceleration badge")
	}
}

func openAIUsageBadgeStandard(ctx context.Context, account *Account, model string) bool {
	if ctx == nil {
		return false
	}
	if snapshot, _ := ctx.Value(openAIGroupTierSnapshotKey{}).(*openAIGroupTierSnapshot); snapshot != nil {
		group, _ := ctx.Value(ctxkey.Group).(*Group)
		return group != nil && group.ID == snapshot.GroupID && snapshot.Source == "group_display_standard"
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	return IsGroupContextValid(group) && group.UsageAccelerationBadge != "" &&
		group.OpenAIModelTiers[openAIGroupRequestedModel(ctx, model)] != "default" &&
		openAIGroupForcedTier(ctx, account, model) == "default"
}
