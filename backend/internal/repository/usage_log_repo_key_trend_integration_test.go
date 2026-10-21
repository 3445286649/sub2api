//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserAPIKeySummaryTrendAndPrevious(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	r := newUsageLogRepositoryWithSQL(client, tx)
	u := mustCreateUser(t, client, &service.User{Email: "key-trend@test.com"})
	g := mustCreateGroup(t, client, &service.Group{Name: "trend-openai", Platform: service.PlatformOpenAI})
	a := mustCreateAccount(t, client, &service.Account{Name: "key-trend-account"})
	work := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "work", Key: "sk-trend-work"})
	other := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "other", Key: "sk-trend-other"})
	deleted := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "gone", Key: "sk-trend-gone"})
	_, err := tx.ExecContext(ctx, "UPDATE api_keys SET group_id=$1 WHERE id=$2", g.ID, work.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE api_keys SET deleted_at=NOW() WHERE id=$1", deleted.ID)
	require.NoError(t, err)
	start := time.Date(2026, 10, 20, 15, 42, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	prevStart, prevEnd := start.Add(-24*time.Hour), start
	insert := func(keyID int64, at time.Time, cost float64) {
		_, err := r.Create(ctx, &service.UsageLog{UserID: u.ID, APIKeyID: keyID, AccountID: a.ID, GroupID: &g.ID, Model: "m", InputTokens: 10, OutputTokens: 5, ActualCost: cost, TotalCost: cost, CreatedAt: at})
		require.NoError(t, err)
	}
	insert(work.ID, start, 1)
	insert(work.ID, start.Add(90*time.Minute), 2)
	insert(work.ID, end.Add(-time.Second), 4)
	insert(other.ID, start.Add(3*time.Hour), 8)
	insert(deleted.ID, start.Add(5*time.Hour), 16)
	insert(work.ID, prevStart.Add(time.Hour), 0.5)
	insert(other.ID, prevStart.Add(time.Hour), 32)

	q := usagestats.APIKeySummaryQuery{UserID: u.ID, StartTime: start, EndTime: end, Page: 1, PageSize: 20, SortBy: "actual_cost",
		APIKeyUsageTrendOptions: usagestats.APIKeyUsageTrendOptions{PrevStartTime: &prevStart, PrevEndTime: &prevEnd, Granularity: usagestats.KeyUsageGranularityHour}}
	got, err := r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.Len(t, got.Trend, 24)
	require.Equal(t, start, got.Trend[0].Bucket.UTC())
	var sum float64
	var reqs int64
	for _, b := range got.Trend {
		sum += b.ActualCost
		reqs += b.Requests
	}
	require.InDelta(t, got.Summary.ActualCost, sum, 1e-9)
	require.Equal(t, got.Summary.Requests, reqs)
	require.InDelta(t, 2, got.Trend[1].ActualCost, 1e-9) // 16:42-17:42 holds the 17:12 entry
	require.InDelta(t, 4, got.Trend[23].ActualCost, 1e-9)
	require.NotNil(t, got.Previous)
	require.InDelta(t, 32.5, got.Previous.ActualCost, 1e-9)
	for _, it := range got.Items {
		if it.APIKeyID == work.ID {
			require.Equal(t, service.PlatformOpenAI, it.GroupPlatform)
		}
	}

	// Search narrows the key set; trend and previous follow the same keys.
	q.Search = "work"
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	sum = 0
	for _, b := range got.Trend {
		sum += b.ActualCost
	}
	require.InDelta(t, 7, got.Summary.ActualCost, 1e-9)
	require.InDelta(t, 7, sum, 1e-9)
	require.InDelta(t, 0.5, got.Previous.ActualCost, 1e-9)

	// No matched keys still returns zero-filled buckets and a zero previous.
	q.Search = "missing"
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.Len(t, got.Trend, 24)
	require.NotNil(t, got.Previous)
	require.Zero(t, got.Previous.Requests)

	// Without prev parameters previous is null; without granularity there is no trend.
	q.Search = ""
	q.APIKeyUsageTrendOptions = usagestats.APIKeyUsageTrendOptions{}
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.Nil(t, got.Previous)
	require.Nil(t, got.Trend)
}

func TestUserAPIKeyBreakdownSeriesDaysAndDST(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	r := newUsageLogRepositoryWithSQL(client, tx)
	u := mustCreateUser(t, client, &service.User{Email: "key-series@test.com"})
	g := mustCreateGroup(t, client, &service.Group{Name: "series-gemini", Platform: service.PlatformGemini})
	g2 := mustCreateGroup(t, client, &service.Group{Name: "series-claude"})
	a := mustCreateAccount(t, client, &service.Account{Name: "key-series-account"})
	k := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "series", Key: "sk-series"})
	_, err := tx.ExecContext(ctx, "UPDATE api_keys SET deleted_at=NOW() WHERE id=$1", k.ID)
	require.NoError(t, err)
	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, 3, 7, 0, 0, 0, 0, ny)
	end := time.Date(2026, 3, 10, 0, 0, 0, 0, ny)
	insert := func(groupID int64, at time.Time, cost float64) {
		_, err := r.Create(ctx, &service.UsageLog{UserID: u.ID, APIKeyID: k.ID, AccountID: a.ID, GroupID: &groupID, Model: "m", InputTokens: 1, ActualCost: cost, TotalCost: cost, CreatedAt: at})
		require.NoError(t, err)
	}
	insert(g.ID, time.Date(2026, 3, 7, 23, 30, 0, 0, ny), 1)
	insert(g.ID, time.Date(2026, 3, 8, 23, 30, 0, 0, ny), 2) // DST day is 23h long
	insert(g2.ID, time.Date(2026, 3, 9, 0, 30, 0, 0, ny), 4)
	insert(g.ID, time.Date(2026, 3, 1, 12, 0, 0, 0, ny), 8) // previous period
	prevStart, prevEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, ny), start

	q := usagestats.APIKeyBreakdownQuery{UserID: u.ID, APIKeyID: k.ID, StartTime: start, EndTime: end,
		APIKeyUsageTrendOptions: usagestats.APIKeyUsageTrendOptions{PrevStartTime: &prevStart, PrevEndTime: &prevEnd, Granularity: usagestats.KeyUsageGranularityDay, Location: ny}}
	got, err := r.GetUserAPIKeyBreakdown(ctx, q)
	require.NoError(t, err)
	require.True(t, got.Deleted)
	require.Len(t, got.Series, 3)
	require.InDelta(t, 1, got.Series[0].ActualCost, 1e-9)
	require.InDelta(t, 2, got.Series[1].ActualCost, 1e-9)
	require.InDelta(t, 4, got.Series[2].ActualCost, 1e-9)
	require.InDelta(t, 8, got.PreviousSummary.ActualCost, 1e-9)
	platforms := map[string]string{}
	for _, grp := range got.Groups {
		platforms[grp.GroupName] = grp.GroupPlatform
	}
	require.Equal(t, service.PlatformGemini, platforms["series-gemini"])
	require.Equal(t, service.PlatformAnthropic, platforms["series-claude"])

	// group filter applies to series and previous.
	q.GroupID = g2.ID
	got, err = r.GetUserAPIKeyBreakdown(ctx, q)
	require.NoError(t, err)
	require.InDelta(t, 0, got.Series[0].ActualCost+got.Series[1].ActualCost, 1e-9)
	require.InDelta(t, 4, got.Series[2].ActualCost, 1e-9)
	require.Zero(t, got.PreviousSummary.Requests)
}
