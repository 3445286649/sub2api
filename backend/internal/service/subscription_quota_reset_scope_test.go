//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type quotaResetUserSubRepoStub struct {
	userSubRepoNoop

	sub         *UserSubscription
	updateCalls int
	updateErr   error
}

func (r *quotaResetUserSubRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func (r *quotaResetUserSubRepoStub) Update(_ context.Context, sub *UserSubscription) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	cp := *sub
	r.sub = &cp
	return nil
}

type quotaResetGroupRepoStub struct {
	groupRepoNoop

	group  *Group
	groups map[int64]*Group
}

func (r *quotaResetGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	if r.groups != nil {
		if group, ok := r.groups[id]; ok {
			cp := *group
			return &cp, nil
		}
	}
	if r.group != nil && r.group.ID == id {
		cp := *r.group
		return &cp, nil
	}
	return nil, ErrGroupNotFound
}

type quotaResetUserSubListRepoStub struct {
	quotaResetUserSubRepoStub

	activeSubs []UserSubscription
}

func (r *quotaResetUserSubListRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	for i := range r.activeSubs {
		sub := r.activeSubs[i]
		if sub.UserID == userID && sub.GroupID == groupID {
			return &sub, nil
		}
	}
	return nil, ErrSubscriptionNotFound
}

func (r *quotaResetUserSubListRepoStub) ListActiveByUserID(_ context.Context, userID int64) ([]UserSubscription, error) {
	out := make([]UserSubscription, 0, len(r.activeSubs))
	for i := range r.activeSubs {
		if r.activeSubs[i].UserID == userID {
			out = append(out, r.activeSubs[i])
		}
	}
	return out, nil
}

func TestResetQuotaUsageDailyRewindsWeeklyAndMonthly(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 34, 56, 0, time.UTC)
	windowStart := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{
			ID:               1,
			UserID:           10,
			GroupID:          20,
			DailyUsageUSD:    35,
			WeeklyUsageUSD:   140,
			MonthlyUsageUSD:  520,
			DailyWindowStart: quotaResetPtrTime(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	sub, err := svc.ResetQuotaUsage(context.Background(), 1, QuotaResetScopeDaily, 0, now)

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, float64(0), sub.DailyUsageUSD)
	require.Equal(t, float64(105), sub.WeeklyUsageUSD)
	require.Equal(t, float64(485), sub.MonthlyUsageUSD)
	require.NotNil(t, sub.DailyWindowStart)
	require.True(t, sub.DailyWindowStart.Equal(windowStart))
}

func TestResetQuotaUsageDailyDoesNotGoNegative(t *testing.T) {
	now := time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC)
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{
			ID:              2,
			UserID:          10,
			GroupID:         20,
			DailyUsageUSD:   100,
			WeeklyUsageUSD:  60,
			MonthlyUsageUSD: 20,
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	sub, err := svc.ResetQuotaUsage(context.Background(), 2, QuotaResetScopeDaily, 0, now)

	require.NoError(t, err)
	require.Equal(t, float64(0), sub.DailyUsageUSD)
	require.Equal(t, float64(0), sub.WeeklyUsageUSD)
	require.Equal(t, float64(0), sub.MonthlyUsageUSD)
}

func TestResetQuotaUsageAllResetsAllWindows(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 15, 0, 0, time.UTC)
	windowStart := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{
			ID:                3,
			UserID:            10,
			GroupID:           20,
			DailyUsageUSD:     5,
			WeeklyUsageUSD:    80,
			MonthlyUsageUSD:   300,
			DailyWindowStart:  quotaResetPtrTime(time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)),
			WeeklyWindowStart: quotaResetPtrTime(time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)),
		},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	sub, err := svc.ResetQuotaUsage(context.Background(), 3, QuotaResetScopeAll, 0, now)

	require.NoError(t, err)
	require.Equal(t, float64(0), sub.DailyUsageUSD)
	require.Equal(t, float64(0), sub.WeeklyUsageUSD)
	require.Equal(t, float64(0), sub.MonthlyUsageUSD)
	require.NotNil(t, sub.DailyWindowStart)
	require.NotNil(t, sub.WeeklyWindowStart)
	require.NotNil(t, sub.MonthlyWindowStart)
	require.True(t, sub.DailyWindowStart.Equal(windowStart))
	require.True(t, sub.WeeklyWindowStart.Equal(windowStart))
	require.True(t, sub.MonthlyWindowStart.Equal(windowStart))
}

func TestResetQuotaUsageRejectsInvalidScope(t *testing.T) {
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{ID: 4, UserID: 10, GroupID: 20},
	}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)

	_, err := svc.ResetQuotaUsage(context.Background(), 4, "invalid", 0, time.Now())

	require.ErrorIs(t, err, ErrInvalidQuotaResetScope)
	require.Equal(t, 0, repo.updateCalls)
}

func TestResetQuotaUsageDailyValueEqualLimitClearsUsage(t *testing.T) {
	now := time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC)
	dailyLimit := 100.0
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{
			ID:              5,
			UserID:          10,
			GroupID:         20,
			DailyUsageUSD:   80,
			WeeklyUsageUSD:  150,
			MonthlyUsageUSD: 300,
		},
	}
	svc := NewSubscriptionService(&quotaResetGroupRepoStub{group: &Group{ID: 20, DailyLimitUSD: &dailyLimit}}, repo, nil, nil, nil)

	sub, err := svc.ResetQuotaUsage(context.Background(), 5, QuotaResetScopeDaily, 100, now)

	require.NoError(t, err)
	require.Equal(t, float64(0), sub.DailyUsageUSD)
	require.Equal(t, float64(70), sub.WeeklyUsageUSD)
	require.Equal(t, float64(220), sub.MonthlyUsageUSD)
}

func TestResetQuotaUsageDailyValueBelowLimitDeductsFixedAmount(t *testing.T) {
	now := time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC)
	dailyLimit := 200.0
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{
			ID:              6,
			UserID:          10,
			GroupID:         20,
			DailyUsageUSD:   180,
			WeeklyUsageUSD:  260,
			MonthlyUsageUSD: 500,
		},
	}
	svc := NewSubscriptionService(&quotaResetGroupRepoStub{group: &Group{ID: 20, DailyLimitUSD: &dailyLimit}}, repo, nil, nil, nil)

	sub, err := svc.ResetQuotaUsage(context.Background(), 6, QuotaResetScopeDaily, 100, now)

	require.NoError(t, err)
	require.Equal(t, float64(80), sub.DailyUsageUSD)
	require.Equal(t, float64(160), sub.WeeklyUsageUSD)
	require.Equal(t, float64(400), sub.MonthlyUsageUSD)
}

func TestResetQuotaUsageDailyValueBelowLimitDoesNotGoNegative(t *testing.T) {
	now := time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC)
	dailyLimit := 200.0
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{
			ID:              7,
			UserID:          10,
			GroupID:         20,
			DailyUsageUSD:   40,
			WeeklyUsageUSD:  60,
			MonthlyUsageUSD: 80,
		},
	}
	svc := NewSubscriptionService(&quotaResetGroupRepoStub{group: &Group{ID: 20, DailyLimitUSD: &dailyLimit}}, repo, nil, nil, nil)

	sub, err := svc.ResetQuotaUsage(context.Background(), 7, QuotaResetScopeDaily, 100, now)

	require.NoError(t, err)
	require.Equal(t, float64(0), sub.DailyUsageUSD)
	require.Equal(t, float64(0), sub.WeeklyUsageUSD)
	require.Equal(t, float64(0), sub.MonthlyUsageUSD)
}

func TestResetQuotaUsageDailyValueAboveLimitRejects(t *testing.T) {
	now := time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC)
	dailyLimit := 100.0
	repo := &quotaResetUserSubRepoStub{
		sub: &UserSubscription{
			ID:             8,
			UserID:         10,
			GroupID:        20,
			DailyUsageUSD:  100,
			WeeklyUsageUSD: 100,
		},
	}
	svc := NewSubscriptionService(&quotaResetGroupRepoStub{group: &Group{ID: 20, DailyLimitUSD: &dailyLimit}}, repo, nil, nil, nil)

	_, err := svc.ResetQuotaUsage(context.Background(), 8, QuotaResetScopeDaily, 200, now)

	require.ErrorIs(t, err, ErrQuotaResetValueExceedsLimit)
	require.Equal(t, 0, repo.updateCalls)
}

func TestResolveQuotaResetSubscriptionValueCanTargetHigherDailyLimitSubscription(t *testing.T) {
	daily100 := 100.0
	daily200 := 200.0
	repo := &quotaResetUserSubListRepoStub{
		activeSubs: []UserSubscription{
			{ID: 20, UserID: 10, GroupID: 13, DailyUsageUSD: 180},
		},
	}
	svc := NewSubscriptionService(&quotaResetGroupRepoStub{groups: map[int64]*Group{
		5:  {ID: 5, DailyLimitUSD: &daily100},
		13: {ID: 13, DailyLimitUSD: &daily200},
	}}, repo, nil, nil, nil)
	redeemSvc := &RedeemService{subscriptionService: svc}

	sub, err := redeemSvc.resolveQuotaResetSubscription(context.Background(), 10, &RedeemCode{
		GroupID:         quotaResetPtrInt64(5),
		Value:           100,
		QuotaResetScope: QuotaResetScopeDaily,
	})

	require.NoError(t, err)
	require.Equal(t, int64(20), sub.ID)
	require.Equal(t, int64(13), sub.GroupID)
}

func TestResolveQuotaResetSubscriptionValueRejectsLowerDailyLimitSubscription(t *testing.T) {
	daily100 := 100.0
	daily200 := 200.0
	repo := &quotaResetUserSubListRepoStub{
		activeSubs: []UserSubscription{
			{ID: 10, UserID: 10, GroupID: 5, DailyUsageUSD: 50},
		},
	}
	svc := NewSubscriptionService(&quotaResetGroupRepoStub{groups: map[int64]*Group{
		5:  {ID: 5, DailyLimitUSD: &daily100},
		13: {ID: 13, DailyLimitUSD: &daily200},
	}}, repo, nil, nil, nil)
	redeemSvc := &RedeemService{subscriptionService: svc}

	_, err := redeemSvc.resolveQuotaResetSubscription(context.Background(), 10, &RedeemCode{
		GroupID:         quotaResetPtrInt64(13),
		Value:           200,
		QuotaResetScope: QuotaResetScopeDaily,
	})

	require.ErrorIs(t, err, ErrQuotaResetValueExceedsLimit)
}

func quotaResetPtrTime(v time.Time) *time.Time {
	return &v
}

func quotaResetPtrInt64(v int64) *int64 {
	return &v
}
