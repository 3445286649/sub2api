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

func TestUserAPIKeySummaryIsolationHistoryAndTotals(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	r := newUsageLogRepositoryWithSQL(client, tx)
	u := mustCreateUser(t, client, &service.User{Email: "key-summary@test.com"})
	other := mustCreateUser(t, client, &service.User{Email: "key-summary-other@test.com"})
	g1 := mustCreateGroup(t, client, &service.Group{Name: "old-group"})
	g2 := mustCreateGroup(t, client, &service.Group{Name: "new-group"})
	a := mustCreateAccount(t, client, &service.Account{Name: "key-summary-account"})
	k1 := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "work", Key: "sk-summary-work"})
	k2 := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "deleted", Key: "sk-summary-deleted"})
	k3 := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "unused", Key: "sk-summary-unused"})
	k4 := mustCreateApiKey(t, client, &service.APIKey{UserID: other.ID, Name: "private-other", Key: "sk-summary-other"})
	_, err := tx.ExecContext(ctx, "UPDATE api_keys SET group_id=$1 WHERE id=$2", g2.ID, k1.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE api_keys SET deleted_at=NOW() WHERE id=$1", k2.ID)
	require.NoError(t, err)
	start := time.Date(2026, 10, 18, 15, 42, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	insert := func(userID, keyID, groupID int64, at time.Time, cost float64, tokens int, billing int8) {
		_, err := r.Create(ctx, &service.UsageLog{UserID: userID, APIKeyID: keyID, AccountID: a.ID, GroupID: &groupID, Model: "test", InputTokens: tokens, OutputTokens: 2, CacheCreationTokens: 3, CacheReadTokens: 4, TotalCost: cost * 2, ActualCost: cost, BillingType: billing, CreatedAt: at})
		require.NoError(t, err)
	}
	insert(u.ID, k1.ID, g1.ID, start, 2, 10, 0)
	insert(u.ID, k1.ID, g2.ID, start.Add(time.Hour), 3, 20, 1)
	insert(u.ID, k2.ID, g1.ID, start.Add(time.Hour), 1, 30, 0)
	insert(other.ID, k4.ID, g1.ID, start.Add(time.Hour), 999, 999, 0)
	insert(u.ID, k1.ID, g1.ID, start.Add(-time.Second), 100, 100, 0)
	insert(u.ID, k1.ID, g1.ID, end, 100, 100, 0)
	// Per-image/per-request billing can have a charge without any token counters.
	_, err = r.Create(ctx, &service.UsageLog{UserID: u.ID, APIKeyID: k1.ID, AccountID: a.ID, GroupID: &g2.ID, Model: "image", ImageCount: 1, TotalCost: 1.4, ActualCost: 0.7, CreatedAt: start.Add(2 * time.Hour)})
	require.NoError(t, err)
	q := usagestats.APIKeySummaryQuery{UserID: u.ID, StartTime: start, EndTime: end, Page: 1, PageSize: 1, SortBy: "actual_cost"}
	got, err := r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 3, got.Total)
	require.Len(t, got.Items, 1)
	require.Equal(t, k1.ID, got.Items[0].APIKeyID)
	require.Equal(t, "new-group", got.Items[0].GroupName)
	require.EqualValues(t, 4, got.Summary.Requests)
	require.EqualValues(t, 87, got.Summary.TotalTokens)
	require.InDelta(t, 6.7, got.Summary.ActualCost, 1e-9)
	require.InDelta(t, 3, got.Summary.SubscriptionCost, 1e-9)
	stats, err := r.GetStatsWithFilters(ctx, usagestats.UsageLogFilters{UserID: u.ID, StartTime: &start, EndTime: &end})
	require.NoError(t, err)
	require.Equal(t, stats.TotalRequests, got.Summary.Requests)
	require.Equal(t, stats.TotalTokens, got.Summary.TotalTokens)
	require.InDelta(t, stats.TotalActualCost, got.Summary.ActualCost, 1e-9)
	q.Page = 2
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.True(t, got.Items[0].Deleted)
	require.Equal(t, k2.ID, got.Items[0].APIKeyID)
	q.Page = 99
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.Empty(t, got.Items)
	require.EqualValues(t, 3, got.Total)
	require.InDelta(t, 6.7, got.Summary.ActualCost, 1e-9)
	q.Page = 1
	q.PageSize = 20
	q.GroupID = g1.ID
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 2, got.Total)
	require.InDelta(t, 3, got.Summary.ActualCost, 1e-9)
	q.GroupID = 0
	q.OnlyUsed = true
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 2, got.Total)
	q.Search = "' OR 1=1 --"
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.Zero(t, got.Total)
	q.Search = ""
	q.APIKeyID = k4.ID
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.Zero(t, got.Total)
	q.APIKeyID = k3.ID
	q.OnlyUsed = false
	got, err = r.GetUserAPIKeySummary(ctx, q)
	require.NoError(t, err)
	require.EqualValues(t, 1, got.Total)
	require.Zero(t, got.Summary.Requests)
	owned, err := r.OwnsHistoricalAPIKey(ctx, u.ID, k2.ID)
	require.NoError(t, err)
	require.True(t, owned)
	owned, err = r.OwnsHistoricalAPIKey(ctx, other.ID, k2.ID)
	require.NoError(t, err)
	require.False(t, owned)
}
