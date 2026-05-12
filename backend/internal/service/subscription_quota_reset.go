package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrInvalidQuotaResetScope = infraerrors.BadRequest("INVALID_QUOTA_RESET_SCOPE", "invalid quota reset scope")
var ErrQuotaResetValueExceedsLimit = infraerrors.BadRequest("QUOTA_RESET_VALUE_EXCEEDS_LIMIT", "quota reset value exceeds subscription limit")

func IsValidQuotaResetScope(scope string) bool {
	switch scope {
	case QuotaResetScopeDaily, QuotaResetScopeWeekly, QuotaResetScopeMonthly, QuotaResetScopeAll:
		return true
	default:
		return false
	}
}

func (s *SubscriptionService) ResetQuotaUsage(ctx context.Context, subscriptionID int64, scope string, resetValueUSD float64, now time.Time) (*UserSubscription, error) {
	if !IsValidQuotaResetScope(scope) {
		return nil, ErrInvalidQuotaResetScope
	}
	if now.IsZero() {
		now = time.Now()
	}
	windowStart := startOfDay(now)

	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	switch scope {
	case QuotaResetScopeDaily:
		deduct, err := s.resolveDailyQuotaResetDeduct(ctx, sub, resetValueUSD)
		if err != nil {
			return nil, err
		}
		sub.DailyUsageUSD = clampUsageFloor(sub.DailyUsageUSD - deduct)
		sub.DailyWindowStart = &windowStart
		sub.WeeklyUsageUSD = clampUsageFloor(sub.WeeklyUsageUSD - deduct)
		sub.MonthlyUsageUSD = clampUsageFloor(sub.MonthlyUsageUSD - deduct)
	case QuotaResetScopeWeekly:
		sub.WeeklyUsageUSD = 0
		sub.WeeklyWindowStart = &windowStart
	case QuotaResetScopeMonthly:
		sub.MonthlyUsageUSD = 0
		sub.MonthlyWindowStart = &windowStart
	case QuotaResetScopeAll:
		sub.DailyUsageUSD = 0
		sub.WeeklyUsageUSD = 0
		sub.MonthlyUsageUSD = 0
		sub.DailyWindowStart = &windowStart
		sub.WeeklyWindowStart = &windowStart
		sub.MonthlyWindowStart = &windowStart
	}

	if err := s.userSubRepo.Update(ctx, sub); err != nil {
		return nil, err
	}

	s.InvalidateSubCache(sub.UserID, sub.GroupID)
	if s.subCacheL1 != nil {
		s.subCacheL1.Wait()
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateSubscription(ctx, sub.UserID, sub.GroupID)
	}

	return s.userSubRepo.GetByID(ctx, subscriptionID)
}

func (s *SubscriptionService) resolveDailyQuotaResetDeduct(ctx context.Context, sub *UserSubscription, resetValueUSD float64) (float64, error) {
	if resetValueUSD <= 0 {
		return sub.DailyUsageUSD, nil
	}

	group, err := s.groupRepo.GetByID(ctx, sub.GroupID)
	if err != nil {
		return 0, err
	}
	if !group.HasDailyLimit() {
		return 0, ErrQuotaResetValueExceedsLimit
	}
	if resetValueUSD > *group.DailyLimitUSD {
		return 0, ErrQuotaResetValueExceedsLimit
	}
	if resetValueUSD >= *group.DailyLimitUSD {
		return sub.DailyUsageUSD, nil
	}
	return resetValueUSD, nil
}

func clampUsageFloor(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
