package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (r apiKeyGroupRepoStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, ErrGroupNotFound
}

func (r apiKeyGroupRepoStub) GetByID(ctx context.Context, id int64) (*Group, error) {
	return r.GetByIDLite(ctx, id)
}

type apiKeyGroupRouterStub map[int64]bool

func (r apiKeyGroupRouterStub) ProbeAPIKeyGroup(_ context.Context, group *Group, _, _ string) (bool, error) {
	return r[group.ID], nil
}

func TestNormalizeAPIKeyGroupIDs(t *testing.T) {
	primary := int64(1)
	ids, err := normalizeAPIKeyGroupIDs(&primary, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{1}, ids)
	for _, invalid := range [][]int64{{1, 1}, {1, 0}, {2, 1}, make([]int64, 21)} {
		_, err := normalizeAPIKeyGroupIDs(&primary, invalid)
		require.Error(t, err)
	}
}

func TestSelectRequestGroupUsesFirstAvailablePermittedGroup(t *testing.T) {
	primary := int64(1)
	groups := map[int64]*Group{
		1: {ID: 1, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"other"}}},
		2: {ID: 2, Status: StatusActive},
		3: {ID: 3, Status: StatusActive},
	}
	key := &APIKey{ID: 10, UserID: 7, GroupID: &primary, GroupIDs: []int64{1, 2, 3},
		Group: groups[1], User: &User{ID: 7, Balance: 5}}
	svc := &APIKeyService{groupRepo: apiKeyGroupRepoStub{groups: groups}, groupRouter: apiKeyGroupRouterStub{2: true, 3: true}}
	selected, err := svc.SelectRequestGroup(context.Background(), key, []string{"target"}, "token", false, true, nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, int64(1), *key.GroupID, "selection must not mutate the cached key")

	groups[2].IsExclusive = true
	selected, err = svc.SelectRequestGroup(context.Background(), key, []string{"target"}, "token", false, true, nil)
	require.NoError(t, err)
	require.Equal(t, int64(3), *selected.GroupID)
}

func TestSelectRequestGroupFailsClosedWithoutAvailableAccount(t *testing.T) {
	primary := int64(1)
	key := &APIKey{UserID: 7, GroupID: &primary, GroupIDs: []int64{1, 2}, User: &User{ID: 7, Balance: 5}}
	svc := &APIKeyService{groupRepo: apiKeyGroupRepoStub{groups: map[int64]*Group{
		1: {ID: 1, Status: StatusActive}, 2: {ID: 2, Status: StatusActive},
	}}, groupRouter: apiKeyGroupRouterStub{}}
	selected, err := svc.SelectRequestGroup(context.Background(), key, []string{"target"}, "token", false, true, nil)
	require.Error(t, err)
	require.Nil(t, selected)
}

func TestValidateAPIKeyGroupsRejectsUnauthorizedAdditionalGroup(t *testing.T) {
	svc := &APIKeyService{groupRepo: apiKeyGroupRepoStub{groups: map[int64]*Group{
		1: {ID: 1, Status: StatusActive},
		2: {ID: 2, Status: StatusActive, IsExclusive: true},
	}}}
	err := svc.validateAPIKeyGroups(context.Background(), &User{ID: 7}, []int64{1, 2})
	require.ErrorIs(t, err, ErrGroupNotAllowed)
}

func TestPinAPIKeyGroupPreservesSelectedBinding(t *testing.T) {
	primary := int64(1)
	group := &Group{ID: 2, Status: StatusActive, RateMultiplier: 0.5}
	svc := &APIKeyService{groupRepo: apiKeyGroupRepoStub{groups: map[int64]*Group{2: group}}}
	key := &APIKey{GroupID: &primary, GroupIDs: []int64{1, 2}, User: &User{ID: 7}}
	selected, err := svc.PinAPIKeyGroup(context.Background(), key, 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Same(t, group, selected.Group)
	require.Equal(t, int64(1), *key.GroupID)

	key.GroupIDs = []int64{1}
	_, err = svc.PinAPIKeyGroup(context.Background(), key, 2)
	require.ErrorIs(t, err, ErrGroupNotAllowed)
}
