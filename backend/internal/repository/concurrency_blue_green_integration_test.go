//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBlueGreenStartupPreservesServingSlots(t *testing.T) {
	ctx := context.Background()
	cache := NewConcurrencyCache(integrationRedis, 10, 60)
	const accountID int64 = 987654321
	const requestID = "old-serving-process-request"
	ok, err := cache.AcquireAccountSlot(ctx, accountID, 1, requestID)
	require.NoError(t, err)
	require.True(t, ok)
	t.Cleanup(func() { _ = cache.ReleaseAccountSlot(ctx, accountID, requestID) })
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.SkipStartupSlotCleanup = true
	service.ProvideConcurrencyService(cache, nil, cfg)
	require.NoError(t, cache.CleanupExpiredAccountSlotKeys(ctx))
	count, err := cache.GetAccountConcurrency(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	ok, err = cache.AcquireAccountSlot(ctx, accountID, 1, "new-candidate-request")
	require.NoError(t, err)
	require.False(t, ok)
	// The default path retains its existing stale-process recovery behavior.
	cfg.Gateway.Scheduling.SkipStartupSlotCleanup = false
	service.ProvideConcurrencyService(cache, nil, cfg)
	count, err = cache.GetAccountConcurrency(ctx, accountID)
	require.NoError(t, err)
	require.Zero(t, count)
}
