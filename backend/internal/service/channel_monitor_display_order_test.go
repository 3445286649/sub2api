//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type displayOrderRepoStub struct {
	duplicateChannelMonitorRepoStub
	updates int
}

func (r *displayOrderRepoStub) Update(_ context.Context, monitor *ChannelMonitor) error {
	r.updates++
	stored := *monitor
	r.source = &stored
	return nil
}

func (r *displayOrderRepoStub) GetByID(ctx context.Context, id int64) (*ChannelMonitor, error) {
	m, err := r.duplicateChannelMonitorRepoStub.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	copy := *m
	return &copy, nil
}

func TestMonitorDisplayOrderCreateUpdate(t *testing.T) {
	ctx := context.Background()
	repo := &displayOrderRepoStub{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	params := ChannelMonitorCreateParams{
		Name: "Plus", Provider: MonitorProviderOpenAI, Endpoint: "https://example.com",
		APIKey: "test-key", PrimaryModel: "model", IntervalSeconds: 60,
		DisplayOrder: 20,
	}
	m, err := svc.Create(ctx, params)
	require.NoError(t, err)
	require.Equal(t, 20, repo.created[0].DisplayOrder)
	repo.source = repo.created[0]
	for _, order := range []int{10, 999999, 0} {
		updated, err := svc.Update(ctx, m.ID, ChannelMonitorUpdateParams{DisplayOrder: &order})
		require.NoError(t, err)
		require.Equal(t, order, updated.DisplayOrder)
		require.Equal(t, params.Provider, updated.Provider)
		require.Equal(t, params.Endpoint, updated.Endpoint)
		updated, err = svc.Update(ctx, m.ID, ChannelMonitorUpdateParams{})
		require.NoError(t, err)
		require.Equal(t, order, updated.DisplayOrder)
	}
	for _, order := range []int{-1, 1000000} {
		params.DisplayOrder = order
		_, err = svc.Create(ctx, params)
		require.Error(t, err)
		require.Len(t, repo.created, 1)
		before := repo.updates
		_, err = svc.Update(ctx, m.ID, ChannelMonitorUpdateParams{DisplayOrder: &order})
		require.Error(t, err)
		require.Equal(t, before, repo.updates)
	}
}
