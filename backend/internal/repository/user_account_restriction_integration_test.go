//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserAccountRestrictionsPersistAndIsolate(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user, err := client.User.Create().SetEmail("account-restriction-test@example.com").SetPasswordHash("test-only").Save(ctx)
	require.NoError(t, err)
	g1, err := client.Group.Create().SetName("restriction-g1").Save(ctx)
	require.NoError(t, err)
	g2, err := client.Group.Create().SetName("restriction-g2").Save(ctx)
	require.NoError(t, err)
	a, err := client.Account.Create().SetName("restriction-account").SetPlatform("openai").SetType("apikey").SetCredentials(map[string]any{}).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", a.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id IN ($1, $2)", g1.ID, g2.ID)
	})
	_, err = integrationDB.ExecContext(ctx, "INSERT INTO account_groups (account_id, group_id) VALUES ($1, $2), ($1, $3)", a.ID, g1.ID, g2.ID)
	require.NoError(t, err)
	repo := NewUserAccountRestrictionRepository(integrationDB)
	require.NoError(t, repo.Replace(ctx, user.ID, g1.ID, []int64{a.ID}))
	require.NoError(t, repo.Replace(ctx, user.ID, g2.ID, []int64{a.ID}))
	require.ErrorIs(t, repo.Replace(ctx, user.ID, g1.ID, []int64{a.ID + 1000000}), service.ErrInvalidAccountRestriction)
	rules, err := repo.GetByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, service.UserAccountRestrictions{g1.ID: {a.ID}, g2.ID: {a.ID}}, rules, "invalid replacement must retain existing rules")
	require.NoError(t, repo.Replace(ctx, user.ID, g1.ID, nil))
	rules, err = repo.GetByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, service.UserAccountRestrictions{g2.ID: {a.ID}}, rules)
}
