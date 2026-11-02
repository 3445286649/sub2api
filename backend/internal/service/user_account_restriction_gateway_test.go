//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserAccountRestrictionGatewayAndGroupProbe(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformGemini, PlatformAntigravity} {
		t.Run(platform, func(t *testing.T) {
			g1, g2 := int64(1), int64(2)
			accounts := []Account{
				{ID: 11, Platform: platform, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 0, AccountGroups: []AccountGroup{{GroupID: g1}, {GroupID: g2}}},
				{ID: 12, Platform: platform, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Priority: 50, AccountGroups: []AccountGroup{{GroupID: g1}, {GroupID: g2}}},
			}
			groups := map[int64]*Group{g1: {ID: g1, Platform: platform, Status: StatusActive, Hydrated: true}, g2: {ID: g2, Platform: platform, Status: StatusActive, Hydrated: true}}
			svc := &GatewayService{accountRepo: newGroupAwareMockRepo(accounts), groupRepo: apiKeyGroupRepoStub{groups: groups}, cache: &mockGatewayCacheForPlatform{}, cfg: testConfig()}
			ctx := WithUserAccountRestrictions(context.Background(), UserAccountRestrictions{g1: {11}})
			account, err := svc.SelectAccountForModel(ctx, &g1, "session", "")
			require.NoError(t, err)
			require.Equal(t, int64(12), account.ID)
			account, err = svc.SelectAccountForModelWithExclusions(ctx, &g1, "session", "", map[int64]struct{}{12: {}})
			require.Error(t, err)
			require.Nil(t, account)
			account, err = svc.SelectAccountForModel(ctx, &g2, "", "")
			require.NoError(t, err)
			require.Equal(t, int64(11), account.ID)

			keyService := &APIKeyService{groupRepo: apiKeyGroupRepoStub{groups: groups}, groupRouter: svc}
			key := &APIKey{UserID: 7, GroupID: &g1, GroupIDs: []int64{g1, g2}, User: &User{ID: 7, Balance: 10, AccountRestrictions: UserAccountRestrictions{g1: {11, 12}}}}
			selected, err := keyService.SelectRequestGroup(context.Background(), key, nil, "token", false, false, nil)
			require.NoError(t, err)
			require.Equal(t, g2, *selected.GroupID, "group probe must skip a pool fully blocked for this user")
			require.Equal(t, g1, *key.GroupID, "cached key group must remain untouched")
		})
	}
}

func TestUserAccountRestrictionBatchImageSelection(t *testing.T) {
	svc, _, _, _, _ := newTestBatchImagePublicService(true)
	svc.AccountRepo = &publicBatchImageAccountRepo{accounts: []Account{
		testBatchImageAccount(11, AccountTypeAPIKey), testBatchImageAccount(12, AccountTypeAPIKey),
	}}
	g1, g2 := int64(1), int64(2)
	owner := testBatchImageOwner()
	owner.GroupID = &g1
	ctx := WithUserAccountRestrictions(context.Background(), UserAccountRestrictions{g1: {11}})
	_, account, err := svc.selectProviderAndAccount(ctx, owner, BatchImageProviderGeminiAPI, "gemini-2.5-flash-image")
	require.NoError(t, err)
	require.Equal(t, int64(12), account.ID)
	owner.GroupID = &g2
	_, account, err = svc.selectProviderAndAccount(ctx, owner, BatchImageProviderGeminiAPI, "gemini-2.5-flash-image")
	require.NoError(t, err)
	require.Equal(t, int64(11), account.ID)
	owner.GroupID = &g1
	ctx = WithUserAccountRestrictions(ctx, UserAccountRestrictions{g1: {11, 12}})
	_, account, err = svc.selectProviderAndAccount(ctx, owner, BatchImageProviderGeminiAPI, "gemini-2.5-flash-image")
	require.ErrorIs(t, err, ErrBatchImageNoAccountAvailable)
	require.Nil(t, account)
}

type restrictionPinnedRepo struct {
	AccountRepository
	accounts []Account
}

func (r restrictionPinnedRepo) ListByGroup(context.Context, int64) ([]Account, error) {
	return r.accounts, nil
}

func TestUserAccountRestrictionPinnedModelAccounts(t *testing.T) {
	svc := &OpenAIGatewayService{accountRepo: restrictionPinnedRepo{accounts: []Account{
		{ID: 11, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true},
		{ID: 12, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true},
	}}}
	group := &Group{ID: 1, Platform: PlatformOpenAI, CodexModelsManifestConfig: GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{11, 12}}}
	ctx := WithUserAccountRestrictions(context.Background(), UserAccountRestrictions{1: {11}})
	results, err := svc.fetchPinnedOpenAIModels(ctx, group, func(_ context.Context, account *Account) (*OpenAIModelsResponse, error) {
		if account.ID != 12 {
			t.Errorf("blocked pinned account was called: %d", account.ID)
		}
		return &OpenAIModelsResponse{Body: []byte(`{"models":[]}`)}, nil
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	ctx = WithUserAccountRestrictions(ctx, UserAccountRestrictions{1: {11, 12}})
	_, err = svc.fetchPinnedOpenAIModels(ctx, group, func(context.Context, *Account) (*OpenAIModelsResponse, error) {
		t.Error("blocked account was called")
		return nil, nil
	})
	require.ErrorIs(t, err, ErrNoPinnedCodexModelsAccounts)
}
