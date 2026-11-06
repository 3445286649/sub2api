//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// renewalModeUserSubRepo adds the extend and notes writes the shared stub leaves unimplemented.
type renewalModeUserSubRepo struct{ *subscriptionUserSubRepoStub }

func (r renewalModeUserSubRepo) ExtendExpiry(_ context.Context, id int64, expiresAt time.Time) error {
	sub := r.byID[id]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.ExpiresAt = expiresAt
	return nil
}

func (r renewalModeUserSubRepo) UpdateNotes(_ context.Context, id int64, notes string) error {
	sub := r.byID[id]
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	sub.Notes = notes
	return nil
}

func newRenewalModeService(now time.Time, sub *UserSubscription) *SubscriptionService {
	groupRepo := &subscriptionGroupRepoStub{group: &Group{ID: sub.GroupID, SubscriptionType: SubscriptionTypeSubscription}}
	stub := newSubscriptionUserSubRepoStub()
	stub.seed(sub)
	svc := NewSubscriptionService(groupRepo, renewalModeUserSubRepo{stub}, nil, nil, nil)
	svc.now = func() time.Time { return now }
	return svc
}

func TestNormalizeSubscriptionRenewalMode(t *testing.T) {
	for raw, want := range map[string]SubscriptionRenewalMode{
		"":          SubscriptionRenewalModeExtend,
		"  ":        SubscriptionRenewalModeExtend,
		"extend":    SubscriptionRenewalModeExtend,
		" restart ": SubscriptionRenewalModeRestart,
	} {
		got, err := NormalizeSubscriptionRenewalMode(raw)
		require.NoError(t, err, raw)
		require.Equal(t, want, got, raw)
	}
	_, err := NormalizeSubscriptionRenewalMode("bogus")
	require.Error(t, err)
}

func TestPaidSubscriptionRestartReplacesTermAndRefillsQuota(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	window := now.AddDate(0, 0, -3)
	svc := newRenewalModeService(now, &UserSubscription{
		ID: 47, UserID: 51, GroupID: 53,
		StartsAt: now.AddDate(0, 0, -10), ExpiresAt: now.AddDate(0, 0, 20),
		Status: SubscriptionStatusActive, Notes: "old",
		DailyWindowStart: &window, WeeklyWindowStart: &window, MonthlyWindowStart: &window,
		DailyUsageUSD: 4, WeeklyUsageUSD: 8, MonthlyUsageUSD: 12,
	})

	sub, renewed, err := svc.assignOrRenewPaidSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 51, GroupID: 53, ValidityDays: 30, Notes: "payment order 9",
	}, SubscriptionRenewalModeRestart, false)

	require.NoError(t, err)
	require.True(t, renewed)
	require.Equal(t, now, sub.StartsAt)
	require.Equal(t, now.AddDate(0, 0, 30), sub.ExpiresAt, "remaining days are replaced, not added")
	require.Equal(t, SubscriptionStatusActive, sub.Status)
	require.Zero(t, sub.DailyUsageUSD)
	require.Zero(t, sub.WeeklyUsageUSD)
	require.Zero(t, sub.MonthlyUsageUSD)
	require.Equal(t, "old\npayment order 9", sub.Notes)
}

func TestPaidSubscriptionExtendKeepsUsageAndAddsDays(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	oldExpiry := now.AddDate(0, 0, 20)
	svc := newRenewalModeService(now, &UserSubscription{
		ID: 47, UserID: 51, GroupID: 53,
		StartsAt: now.AddDate(0, 0, -10), ExpiresAt: oldExpiry,
		Status: SubscriptionStatusActive, Notes: "old",
		DailyUsageUSD: 4, WeeklyUsageUSD: 8, MonthlyUsageUSD: 12,
	})

	sub, renewed, err := svc.assignOrRenewPaidSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 51, GroupID: 53, ValidityDays: 30, Notes: "payment order 9",
	}, SubscriptionRenewalModeExtend, false)

	require.NoError(t, err)
	require.True(t, renewed)
	require.Equal(t, oldExpiry.AddDate(0, 0, 30), sub.ExpiresAt)
	require.Equal(t, now.AddDate(0, 0, -10), sub.StartsAt)
	require.Equal(t, float64(4), sub.DailyUsageUSD)
	require.Equal(t, float64(8), sub.WeeklyUsageUSD)
	require.Equal(t, float64(12), sub.MonthlyUsageUSD)
}

func TestPaidSubscriptionRejectsUnknownRenewalMode(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	svc := newRenewalModeService(now, &UserSubscription{
		ID: 47, UserID: 51, GroupID: 53, StartsAt: now, ExpiresAt: now.AddDate(0, 0, 5), Status: SubscriptionStatusActive,
	})
	_, _, err := svc.assignOrRenewPaidSubscription(context.Background(), &AssignSubscriptionInput{
		UserID: 51, GroupID: 53, ValidityDays: 30,
	}, SubscriptionRenewalMode("bogus"), false)
	require.Error(t, err)
}
