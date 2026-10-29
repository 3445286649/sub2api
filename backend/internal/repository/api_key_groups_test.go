package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepositoryAdditionalGroupsRoundTrip(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "additional-groups@test.com")
	first, err := client.Group.Create().SetName("primary").Save(ctx)
	require.NoError(t, err)
	second, err := client.Group.Create().SetName("backup").Save(ctx)
	require.NoError(t, err)
	third, err := client.Group.Create().SetName("second backup").Save(ctx)
	require.NoError(t, err)
	key := &service.APIKey{UserID: user.ID, Key: "sk-multi-group-roundtrip", Name: "multi", Status: service.StatusActive,
		GroupID: &first.ID, GroupIDs: []int64{first.ID, third.ID, second.ID}}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, []int64{first.ID, third.ID, second.ID}, got.GroupIDs)
	keys, page, err := repo.ListByUserID(ctx, user.ID, pagination.PaginationParams{Page: 1, PageSize: 10}, service.APIKeyListFilters{GroupID: &second.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Equal(t, []int64{first.ID, third.ID, second.ID}, keys[0].GroupIDs)

	got.GroupID, got.GroupIDs = &second.ID, []int64{second.ID}
	require.NoError(t, repo.Update(ctx, got, service.APIKeyUpdateFields{GroupID: true, GroupIDs: true}))
	updated, err := repo.GetByID(ctx, got.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{second.ID}, updated.GroupIDs)
}

func TestAPIKeyRepositoryAdditionalGroupFailureRollsBack(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "additional-groups-rollback@test.com")
	first, err := client.Group.Create().SetName("valid").Save(ctx)
	require.NoError(t, err)
	key := &service.APIKey{UserID: user.ID, Key: "sk-multi-group-rollback", Name: "multi", Status: service.StatusActive,
		GroupID: &first.ID, GroupIDs: []int64{first.ID, 999999}}
	require.Error(t, repo.Create(ctx, key))
	exists, err := repo.ExistsByKey(ctx, key.Key)
	require.NoError(t, err)
	require.False(t, exists)
}
