package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetByKeyForAuth_OpenAIModelTiersStayInTheirGroup(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "openai-tiers-auth@test.com")

	createKey := func(name string, ultrafast bool) string {
		builder := client.Group.Create().
			SetName(name).
			SetPlatform(service.PlatformOpenAI).
			SetStatus(service.StatusActive).
			SetSubscriptionType(service.SubscriptionTypeStandard).
			SetRateMultiplier(1)
		if ultrafast {
			builder.SetForceOpenaiUltrafast(true).
				SetOpenaiModelTiers(map[string]string{"gpt-6.1-sol": "ultrafast"})
		}
		group, err := builder.Save(ctx)
		require.NoError(t, err)
		key := &service.APIKey{
			UserID: user.ID, Key: "sk-" + name, Name: name,
			GroupID: &group.ID, Status: service.StatusActive,
		}
		require.NoError(t, repo.Create(ctx, key))
		return key.Key
	}

	testKey := createKey("ultrafast-group", true)
	otherKey := createKey("standard-group", false)

	testAuth, err := repo.GetByKeyForAuth(ctx, testKey)
	require.NoError(t, err)
	require.True(t, testAuth.Group.ForceOpenAIUltrafast)
	require.Equal(t, "ultrafast", testAuth.Group.OpenAIModelTiers["gpt-6.1-sol"])

	otherAuth, err := repo.GetByKeyForAuth(ctx, otherKey)
	require.NoError(t, err)
	require.False(t, otherAuth.Group.ForceOpenAIUltrafast)
	require.Empty(t, otherAuth.Group.OpenAIModelTiers)
}
