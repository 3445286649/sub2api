package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type restrictionAuthRepo struct{ APIKeyRepository }

func (restrictionAuthRepo) GetByKeyForAuth(_ context.Context, key string) (*APIKey, error) {
	return &APIKey{UserID: 7, Key: key, User: &User{ID: 7}}, nil
}
func (restrictionAuthRepo) ListKeysByUserID(context.Context, int64) ([]string, error) {
	return []string{"one", "two"}, nil
}

type restrictionAuthStore struct {
	UserAccountRestrictionRepository
	rules UserAccountRestrictions
	err   error
	reads int
}

func (s *restrictionAuthStore) GetByUserID(context.Context, int64) (UserAccountRestrictions, error) {
	s.reads++
	return cloneUserAccountRestrictions(s.rules), s.err
}

type restrictionAuthCache struct {
	APIKeyCache
	entries map[string]*APIKeyAuthCacheEntry
}

func (c *restrictionAuthCache) GetAuthCache(_ context.Context, key string) (*APIKeyAuthCacheEntry, error) {
	return c.entries[key], nil
}
func (c *restrictionAuthCache) SetAuthCache(_ context.Context, key string, entry *APIKeyAuthCacheEntry, _ time.Duration) error {
	c.entries[key] = entry
	return nil
}
func (c *restrictionAuthCache) DeleteAuthCache(_ context.Context, key string) error {
	delete(c.entries, key)
	return nil
}
func (c *restrictionAuthCache) PublishAuthCacheInvalidation(context.Context, string) error {
	return nil
}

func TestUserAccountRestrictionAuthenticationCacheAndAllKeys(t *testing.T) {
	ctx := context.Background()
	store := &restrictionAuthStore{rules: UserAccountRestrictions{1: {11}}}
	cache := &restrictionAuthCache{entries: make(map[string]*APIKeyAuthCacheEntry)}
	cfg := &config.Config{APIKeyAuth: config.APIKeyAuthCacheConfig{L2TTLSeconds: 60}}
	svc := NewAPIKeyService(restrictionAuthRepo{}, nil, nil, nil, nil, cache, cfg)
	svc.accountRestrictionRepo = store
	for _, key := range []string{"one", "two"} {
		loaded, err := svc.GetByKey(ctx, key)
		require.NoError(t, err)
		require.Equal(t, []int64{11}, loaded.User.AccountRestrictions[1])
	}
	_, err := svc.GetByKey(ctx, "one")
	require.NoError(t, err)
	require.Equal(t, 2, store.reads, "cache hits must not query restrictions again")
	store.rules = UserAccountRestrictions{1: {12}, 2: {22}}
	svc.InvalidateAuthCacheByUserID(ctx, 7)
	for _, key := range []string{"one", "two"} {
		loaded, err := svc.GetByKey(ctx, key)
		require.NoError(t, err)
		require.Equal(t, store.rules, loaded.User.AccountRestrictions)
	}
	svc.InvalidateAuthCacheByUserID(ctx, 7)
	store.err = errors.New("restriction database unavailable")
	loaded, err := svc.GetByKey(ctx, "one")
	require.Error(t, err, "a failed permission read must not allow unrestricted access")
	require.Nil(t, loaded)
}
