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

func TestUserAPIKeyBreakdownHistoryModelsTotalsAndIsolation(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	r := newUsageLogRepositoryWithSQL(client, tx)
	u := mustCreateUser(t, client, &service.User{Email: "key-breakdown@test.com"})
	other := mustCreateUser(t, client, &service.User{Email: "key-breakdown-other@test.com"})
	g1 := mustCreateGroup(t, client, &service.Group{Name: "previous-group"})
	g2 := mustCreateGroup(t, client, &service.Group{Name: "composite-group", Platform: service.PlatformComposite})
	a := mustCreateAccount(t, client, &service.Account{Name: "key-breakdown-account"})
	k := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "work", Key: "sk-breakdown-work"})
	unused := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Name: "unused", Key: "sk-breakdown-unused"})
	foreign := mustCreateApiKey(t, client, &service.APIKey{UserID: other.ID, Name: "other", Key: "sk-breakdown-other"})
	start := time.Date(2026, 10, 18, 15, 42, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	insert := func(groupID *int64, model, requested string, at time.Time, cost float64, tokens int, billing int8) {
		_, err := r.Create(ctx, &service.UsageLog{
			UserID: u.ID, APIKeyID: k.ID, AccountID: a.ID, GroupID: groupID,
			Model: model, RequestedModel: requested, UpstreamModel: stringPointer("private-upstream"),
			InputTokens: tokens, OutputTokens: 2, CacheCreationTokens: 3, CacheReadTokens: 4,
			TotalCost: cost * 2, ActualCost: cost, BillingType: billing, CreatedAt: at,
		})
		require.NoError(t, err)
	}
	insert(&g1.ID, "legacy-one", "public-alias", start, 2, 10, 1)
	insert(&g1.ID, "legacy-two", " public-alias ", start.Add(time.Minute), 1, 20, 0)
	insert(&g2.ID, "mapped", "public-alias", start.Add(time.Hour), 4, 40, 0)
	insert(nil, "legacy-unknown", "", start.Add(time.Hour), 0.1, 1, 0)
	insert(&g1.ID, "excluded-before", "", start.Add(-time.Microsecond), 100, 100, 0)
	insert(&g1.ID, "excluded-end", "", end, 100, 100, 0)
	for _, log := range []*service.UsageLog{
		{UserID: u.ID, APIKeyID: k.ID, AccountID: a.ID, GroupID: &g1.ID, Model: "free-model", CreatedAt: start.Add(time.Hour)},
		{UserID: u.ID, APIKeyID: k.ID, AccountID: a.ID, GroupID: &g2.ID, Model: "image", ImageCount: 1, ActualCost: 0.7, CreatedAt: start.Add(time.Hour)},
		{UserID: other.ID, APIKeyID: foreign.ID, AccountID: a.ID, GroupID: &g1.ID, Model: "private", ActualCost: 999, CreatedAt: start.Add(time.Hour)},
		{UserID: u.ID, APIKeyID: unused.ID, AccountID: a.ID, GroupID: &g1.ID, Model: "other-key", ActualCost: 999, CreatedAt: start.Add(time.Hour)},
	} {
		_, err := r.Create(ctx, log)
		require.NoError(t, err)
	}
	_, err := tx.ExecContext(ctx, "UPDATE usage_logs SET requested_model=NULL WHERE api_key_id=$1 AND model='legacy-unknown'", k.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE usage_logs SET requested_model=' public-alias ' WHERE api_key_id=$1 AND model='legacy-two'", k.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE api_keys SET group_id=$1, deleted_at=NOW() WHERE id=$2", g2.ID, k.ID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE groups SET name='renamed-old-group', deleted_at=NOW() WHERE id=$1", g1.ID)
	require.NoError(t, err)
	q := usagestats.APIKeyBreakdownQuery{UserID: u.ID, APIKeyID: k.ID, StartTime: start, EndTime: end}
	got, err := r.GetUserAPIKeyBreakdown(ctx, q)
	require.NoError(t, err)
	require.Equal(t, k.ID, got.APIKeyID)
	require.Equal(t, "work", got.Name)
	require.True(t, got.Deleted)
	require.Equal(t, start, got.StartTime)
	require.Equal(t, end, got.EndTime)
	require.EqualValues(t, 6, got.Summary.Requests)
	require.EqualValues(t, 107, got.Summary.TotalTokens)
	require.InDelta(t, 7.8, got.Summary.ActualCost, 1e-9)
	require.InDelta(t, 2, got.Summary.SubscriptionCost, 1e-9)
	require.Len(t, got.Groups, 3)
	require.Equal(t, g2.ID, *got.Groups[0].GroupID)
	require.False(t, got.Groups[0].GroupDeleted)
	require.Equal(t, "public-alias", got.Groups[0].Models[0].Model)
	require.Equal(t, "image", got.Groups[0].Models[1].Model)
	require.Zero(t, got.Groups[0].Models[1].TotalTokens)
	require.Equal(t, g1.ID, *got.Groups[1].GroupID)
	require.Equal(t, "renamed-old-group", got.Groups[1].GroupName)
	require.True(t, got.Groups[1].GroupDeleted)
	require.Equal(t, "public-alias", got.Groups[1].Models[0].Model)
	require.EqualValues(t, 2, got.Groups[1].Models[0].Requests)
	require.EqualValues(t, 48, got.Groups[1].Models[0].TotalTokens)
	require.Equal(t, "free-model", got.Groups[1].Models[1].Model)
	require.Zero(t, got.Groups[1].Models[1].ActualCost)
	require.Nil(t, got.Groups[2].GroupID)
	require.Equal(t, "legacy-unknown", got.Groups[2].Models[0].Model)
	assertKeyBreakdownTotals(t, got)

	// The same key/time/group filters match the existing summary, including subscriptions.
	summary, err := r.GetUserAPIKeySummary(ctx, usagestats.APIKeySummaryQuery{UserID: u.ID, APIKeyID: k.ID, StartTime: start, EndTime: end, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, summary.Summary, got.Summary)
	q.GroupID = g1.ID
	got, err = r.GetUserAPIKeyBreakdown(ctx, q)
	require.NoError(t, err)
	require.Len(t, got.Groups, 1)
	require.EqualValues(t, 3, got.Summary.Requests)
	require.InDelta(t, 3, got.Summary.ActualCost, 1e-9)
	assertKeyBreakdownTotals(t, got)

	q.GroupID = 0
	q.StartTime, q.EndTime = end, end.Add(time.Hour)
	q.APIKeyID = unused.ID
	got, err = r.GetUserAPIKeyBreakdown(ctx, q)
	require.NoError(t, err)
	require.Empty(t, got.Groups)
	require.NotNil(t, got.Groups)
	require.Equal(t, usagestats.APIKeyUsageTotals{}, got.Summary)
	q.APIKeyID = foreign.ID
	_, err = r.GetUserAPIKeyBreakdown(ctx, q)
	require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
	q.APIKeyID = k.ID
	q.UserID = other.ID
	_, err = r.GetUserAPIKeyBreakdown(ctx, q)
	require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
}

func assertKeyBreakdownTotals(t *testing.T, result *usagestats.APIKeyUsageBreakdown) {
	t.Helper()
	add := func(dst *usagestats.APIKeyUsageTotals, src usagestats.APIKeyUsageTotals) {
		dst.Requests += src.Requests
		dst.InputTokens += src.InputTokens
		dst.OutputTokens += src.OutputTokens
		dst.CacheCreationTokens += src.CacheCreationTokens
		dst.CacheReadTokens += src.CacheReadTokens
		dst.TotalTokens += src.TotalTokens
		dst.ActualCost += src.ActualCost
		dst.SubscriptionCost += src.SubscriptionCost
	}
	compare := func(want, got usagestats.APIKeyUsageTotals) {
		require.InDelta(t, want.ActualCost, got.ActualCost, 1e-9)
		require.InDelta(t, want.SubscriptionCost, got.SubscriptionCost, 1e-9)
		want.ActualCost, got.ActualCost, want.SubscriptionCost, got.SubscriptionCost = 0, 0, 0, 0
		require.Equal(t, want, got)
	}
	var total usagestats.APIKeyUsageTotals
	for _, group := range result.Groups {
		var models usagestats.APIKeyUsageTotals
		for _, model := range group.Models {
			add(&models, model.APIKeyUsageTotals)
		}
		compare(group.APIKeyUsageTotals, models)
		add(&total, group.APIKeyUsageTotals)
	}
	compare(result.Summary, total)
}
