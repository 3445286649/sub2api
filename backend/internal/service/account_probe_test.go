package service

import (
	"context"
	"sort"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type memoryAccountProbeRepo struct {
	events      []AccountHealthEvent
	nextProbeAt map[int64]*time.Time
	cacheUsage  AccountProbeCacheUsage
}

func (r *memoryAccountProbeRepo) ClaimDueProbe(_ context.Context, accountID int64, now, leaseUntil time.Time) (bool, error) {
	if next := r.nextProbeAt[accountID]; next != nil && next.After(now) {
		return false, nil
	}
	return true, r.ScheduleNextProbe(context.Background(), accountID, &leaseUntil, now)
}

func (r *memoryAccountProbeRepo) ScheduleNextProbe(_ context.Context, accountID int64, nextProbeAt *time.Time, _ time.Time) error {
	if r.nextProbeAt == nil {
		r.nextProbeAt = make(map[int64]*time.Time)
	}
	r.nextProbeAt[accountID] = nextProbeAt
	return nil
}

func (r *memoryAccountProbeRepo) GetNextProbeAt(_ context.Context, accountID int64) (*time.Time, error) {
	return r.nextProbeAt[accountID], nil
}

func (r *memoryAccountProbeRepo) InsertEvent(_ context.Context, event *AccountHealthEvent) error {
	cp := *event
	cp.ID = int64(len(r.events) + 1)
	r.events = append(r.events, cp)
	return nil
}

func (r *memoryAccountProbeRepo) ListProbeEvents(_ context.Context, accountIDs []int64, since time.Time) ([]AccountHealthEvent, error) {
	allowed := make(map[int64]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		allowed[id] = struct{}{}
	}
	out := make([]AccountHealthEvent, 0)
	for _, event := range r.events {
		_, accountAllowed := allowed[event.AccountID]
		probeSource := event.Source == AccountHealthEventSourceBackgroundProbe || event.Source == AccountHealthEventSourceManualProbe
		probeResult := event.EventType == AccountHealthEventTypeSuccess || event.EventType == AccountHealthEventTypeFailure
		if accountAllowed && probeSource && probeResult && !event.CreatedAt.Before(since) {
			out = append(out, event)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *memoryAccountProbeRepo) ListEvents(_ context.Context, accountID int64, eventType string, since time.Time, params pagination.PaginationParams) (*AccountHealthEventList, error) {
	items := make([]AccountHealthEvent, 0)
	for _, event := range r.events {
		if event.AccountID == accountID && !event.CreatedAt.Before(since) && (eventType == "" || event.EventType == eventType) {
			items = append(items, event)
		}
	}
	return &AccountHealthEventList{Items: items, Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize}, nil
}

func (r *memoryAccountProbeRepo) GetRecentCacheUsage(_ context.Context, _ int64, _, _ time.Time) (*AccountProbeCacheUsage, error) {
	usage := r.cacheUsage
	return &usage, nil
}

func (r *memoryAccountProbeRepo) DeleteEventsBefore(_ context.Context, before time.Time) (int64, error) {
	kept := r.events[:0]
	var deleted int64
	for _, event := range r.events {
		if event.CreatedAt.Before(before) {
			deleted++
			continue
		}
		kept = append(kept, event)
	}
	r.events = kept
	return deleted, nil
}

type accountProbeAccountRepoStub struct {
	accounts map[int64]*Account
}

func (r *accountProbeAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	return r.accounts[id], nil
}

func (r *accountProbeAccountRepoStub) ListHealthyProbeCandidates(_ context.Context, _ time.Time, _ int) ([]Account, error) {
	out := make([]Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		out = append(out, *account)
	}
	return out, nil
}

func TestAccountProbeFailureIsObservationOnly(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	interval := 5
	account := &Account{
		ID:                         42,
		Status:                     StatusActive,
		Schedulable:                true,
		HealthProbeEnabled:         true,
		HealthProbeIntervalMinutes: &interval,
	}
	repo := &memoryAccountProbeRepo{}
	svc := &AccountHealthService{
		repo:        repo,
		accountRepo: &accountProbeAccountRepoStub{accounts: map[int64]*Account{account.ID: account}},
		now:         func() time.Time { return now },
	}

	for range 5 {
		require.NoError(t, svc.RecordProbeFailure(ctx, account.ID, "upstream_5xx", "temporary upstream failure"))
	}

	require.True(t, account.Schedulable)
	require.Nil(t, account.TempUnschedulableUntil)
	require.Empty(t, account.TempUnschedulableReason)
	require.Len(t, repo.events, 5)
	for _, event := range repo.events {
		require.Equal(t, AccountHealthEventSourceBackgroundProbe, event.Source)
		require.Equal(t, AccountHealthEventTypeFailure, event.EventType)
		require.Nil(t, event.LatencyMs)
	}
	require.Equal(t, now.Add(5*time.Minute), *repo.nextProbeAt[account.ID])
}

func TestAccountProbeTrendUsesOnlyProbeEventsAndKeepsFailureLatencyEmpty(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	latency100 := int64(100)
	latency300 := int64(300)
	repo := &memoryAccountProbeRepo{events: []AccountHealthEvent{
		{AccountID: 7, Source: "real_request", EventType: AccountHealthEventTypeSuccess, LatencyMs: &latency100, CreatedAt: now.Add(-4 * time.Hour)},
		{AccountID: 7, Source: AccountHealthEventSourceBackgroundProbe, EventType: AccountHealthEventTypeSuccess, LatencyMs: &latency100, CreatedAt: now.Add(-3 * time.Hour)},
		{AccountID: 7, Source: AccountHealthEventSourceBackgroundProbe, EventType: AccountHealthEventTypeFailure, ErrorCategory: "upstream_5xx", CreatedAt: now.Add(-2 * time.Hour)},
		{AccountID: 7, Source: AccountHealthEventSourceManualProbe, EventType: AccountHealthEventTypeSuccess, LatencyMs: &latency300, CreatedAt: now.Add(-time.Hour)},
	}}
	svc := &AccountHealthService{repo: repo, now: func() time.Time { return now }}

	trends, err := svc.GetProbeTrends(context.Background(), []int64{7})
	require.NoError(t, err)
	require.Len(t, trends, 1)
	trend := trends[0]
	require.Equal(t, 3, trend.Total)
	require.Equal(t, 2, trend.SuccessCount)
	require.Equal(t, 1, trend.FailureCount)
	require.InDelta(t, 66.666, *trend.SuccessRate, 0.01)
	require.Equal(t, int64(100), *trend.P50LatencyMs)
	require.Equal(t, int64(300), *trend.P95LatencyMs)
	require.Len(t, trend.Points, 3)
	require.Nil(t, trend.Points[1].LatencyMs)
	require.Equal(t, 1, trend.Points[1].FailureCount)
}

func TestAccountProbeDetailUses24HoursAndCalculatesOneHourCacheRate(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	latency := int64(250)
	account := &Account{ID: 9, Status: StatusActive}
	repo := &memoryAccountProbeRepo{
		events: []AccountHealthEvent{
			{AccountID: 9, Source: AccountHealthEventSourceBackgroundProbe, EventType: AccountHealthEventTypeSuccess, LatencyMs: &latency, CreatedAt: now.Add(-2 * time.Hour)},
			{AccountID: 9, Source: AccountHealthEventSourceBackgroundProbe, EventType: AccountHealthEventTypeFailure, CreatedAt: now.Add(-25 * time.Hour)},
		},
		cacheUsage: AccountProbeCacheUsage{RequestCount: 3, InputTokens: 50, CacheCreationTokens: 25, CacheReadTokens: 25},
	}
	svc := &AccountHealthService{
		repo:        repo,
		accountRepo: &accountProbeAccountRepoStub{accounts: map[int64]*Account{9: account}},
		now:         func() time.Time { return now },
	}

	detail, err := svc.GetProbeDetail(context.Background(), 9)
	require.NoError(t, err)
	require.Len(t, detail.Points, 1)
	require.Equal(t, 1, detail.Points[0].SuccessCount)
	require.Equal(t, latency, *detail.Points[0].LatencyMs)
	require.Equal(t, "24h", detail.Range)
	require.Equal(t, "1h", detail.CacheStats.Window)
	require.Equal(t, int64(3), detail.CacheStats.RequestCount)
	require.InDelta(t, 25.0, *detail.CacheStats.CacheRate, 0.001)
}

func TestAccountProbeEventListOnlyReturnsLast24Hours(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	repo := &memoryAccountProbeRepo{events: []AccountHealthEvent{
		{AccountID: 9, Source: AccountHealthEventSourceBackgroundProbe, EventType: AccountHealthEventTypeSuccess, CreatedAt: now.Add(-23 * time.Hour)},
		{AccountID: 9, Source: AccountHealthEventSourceBackgroundProbe, EventType: AccountHealthEventTypeFailure, CreatedAt: now.Add(-25 * time.Hour)},
	}}
	svc := &AccountHealthService{repo: repo, now: func() time.Time { return now }}

	page, err := svc.ListEvents(context.Background(), 9, "", pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, AccountHealthEventTypeSuccess, page.Items[0].EventType)
}

func TestAccountProbeCleanupKeepsFortyEightHourSafetyWindow(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	repo := &memoryAccountProbeRepo{events: []AccountHealthEvent{
		{AccountID: 9, CreatedAt: now.Add(-49 * time.Hour)},
		{AccountID: 9, CreatedAt: now.Add(-47 * time.Hour)},
	}}
	runner := NewAccountHealthEventCleanupRunner(&AccountHealthService{repo: repo})
	runner.now = func() time.Time { return now }

	runner.cleanupOnce()

	require.Len(t, repo.events, 1)
	require.Equal(t, now.Add(-47*time.Hour), repo.events[0].CreatedAt)
}

func TestAccountProbeTrendsLimitListPointsWithoutChangingSummary(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	latency := int64(250)
	events := make([]AccountHealthEvent, 30)
	for i := range events {
		events[i] = AccountHealthEvent{
			AccountID: 11,
			Source:    AccountHealthEventSourceBackgroundProbe,
			EventType: AccountHealthEventTypeSuccess,
			LatencyMs: &latency,
			CreatedAt: now.Add(time.Duration(i-30) * time.Minute),
		}
	}
	svc := &AccountHealthService{repo: &memoryAccountProbeRepo{events: events}, now: func() time.Time { return now }}

	trends, err := svc.GetProbeTrends(context.Background(), []int64{11})
	require.NoError(t, err)
	require.Len(t, trends, 1)
	require.Equal(t, 30, trends[0].Total)
	require.Len(t, trends[0].Points, accountProbeListPointLimit)
	require.Equal(t, events[20].CreatedAt, trends[0].Points[0].Timestamp)
}

func TestAccountProbeFailureRedactsSensitiveErrorText(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	account := &Account{ID: 12, Status: StatusActive, Schedulable: true, HealthProbeEnabled: true}
	repo := &memoryAccountProbeRepo{}
	svc := &AccountHealthService{
		repo:        repo,
		accountRepo: &accountProbeAccountRepoStub{accounts: map[int64]*Account{12: account}},
		now:         func() time.Time { return now },
	}

	require.NoError(t, svc.RecordManualProbeFailure(context.Background(), 12, "auth_error", "authorization: Bearer secret-value", nil))
	require.Len(t, repo.events, 1)
	require.NotContains(t, repo.events[0].ErrorMessage, "secret-value")
}

func TestAccountProbeCanScheduleWhenManuallyUnschedulable(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	account := &Account{
		ID: 13, Status: StatusActive, Schedulable: false, HealthProbeEnabled: true,
		Extra: map[string]any{AccountHealthProbeWhenUnschedulableExtraKey: true},
	}
	next := nextScheduledAccountProbe(account, now)
	require.NotNil(t, next)
	require.Equal(t, now.Add(accountProbeDefaultInterval), *next)
}

func TestTruncateAccountProbeStringKeepsValidUTF8(t *testing.T) {
	value := truncateAccountProbeString("上游返回异常", 5)
	require.Equal(t, "上", value)
	require.True(t, utf8.ValidString(value))
}
