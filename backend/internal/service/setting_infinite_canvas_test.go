package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestInfiniteCanvasSettingsDefaultOffAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &usageRebateSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.InfiniteCanvasEnabled)
	for _, enabled := range []bool{true, false} {
		settings.InfiniteCanvasEnabled = enabled
		require.NoError(t, svc.UpdateSettings(ctx, settings))
		repo.values = repo.updates
		got, err := svc.GetAllSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, got.InfiniteCanvasEnabled)
		public, err := svc.GetPublicSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, public.InfiniteCanvasEnabled)
		injected, err := svc.GetPublicSettingsForInjection(ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, injected.(*PublicSettingsInjectionPayload).InfiniteCanvasEnabled)
	}
}
