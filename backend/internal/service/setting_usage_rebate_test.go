package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type usageRebateSettingRepoStub struct {
	values  map[string]string
	updates map[string]string
}

func (s *usageRebateSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (s *usageRebateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (s *usageRebateSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *usageRebateSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := map[string]string{}
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}
func (s *usageRebateSettingRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	s.updates = map[string]string{}
	for key, value := range values {
		s.updates[key] = value
	}
	return nil
}
func (s *usageRebateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	result := map[string]string{}
	for key, value := range s.values {
		result[key] = value
	}
	return result, nil
}
func (s *usageRebateSettingRepoStub) Delete(context.Context, string) error { return nil }

func TestUsageRebateSettingDefaultsOffAndPersistsExplicitState(t *testing.T) {
	repo := &usageRebateSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.UsageRebateEnabled)
	require.False(t, svc.IsUsageRebateEnabled(context.Background()))

	settings.UsageRebateEnabled = true
	require.NoError(t, svc.UpdateSettings(context.Background(), settings))
	require.Equal(t, "true", repo.updates[SettingKeyUsageRebateEnabled])

	repo.values[SettingKeyUsageRebateEnabled] = "true"
	publicSettings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, publicSettings.UsageRebateEnabled)
}
