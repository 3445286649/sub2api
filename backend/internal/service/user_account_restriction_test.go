package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestUserAccountRestrictionOpenAIScheduling(t *testing.T) {
	for _, advanced := range []string{"false", "true"} {
		for _, loadBatch := range []bool{false, true} {
			t.Run(advanced+"/loadBatch="+map[bool]string{false: "false", true: "true"}[loadBatch], func(t *testing.T) {
				resetOpenAIAdvancedSchedulerSettingCacheForTest()
				defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
				g1, g2 := int64(1), int64(2)
				accounts := []Account{
					{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{g1, g2}},
					{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{g1, g2}},
				}
				cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:restricted-session": 11}}
				cfg := &config.Config{}
				cfg.Gateway.Scheduling.LoadBatchEnabled = loadBatch
				cfg.Gateway.OpenAIWS.LBTopK = 2
				svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: cache, cfg: cfg,
					rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(advanced, "false"),
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
				ctx := WithUserAccountRestrictions(context.Background(), UserAccountRestrictions{g1: {11}})
				result, _, err := svc.SelectAccountWithScheduler(ctx, &g1, "", "restricted-session", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, int64(12), result.Account.ID, "sticky account must be skipped")
				if result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
				result, _, err = svc.SelectAccountWithScheduler(ctx, &g1, "", "", "gpt-5.1", map[int64]struct{}{12: {}}, OpenAIUpstreamTransportAny, false)
				require.ErrorIs(t, err, ErrNoAvailableAccounts, "retry cannot resurrect a blocked account")
				require.Nil(t, result)
				result, _, err = svc.SelectAccountWithScheduler(ctx, &g2, "", "", "gpt-5.1", map[int64]struct{}{12: {}}, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, int64(11), result.Account.ID, "same account remains available in another group")
				if result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
				result, _, err = svc.SelectAccountWithScheduler(context.Background(), &g1, "", "", "gpt-5.1", map[int64]struct{}{12: {}}, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, int64(11), result.Account.ID, "other user remains unaffected")
				if result.ReleaseFunc != nil {
					result.ReleaseFunc()
				}
			})
		}
	}
}

func TestUserAccountRestrictionsScopeAndSchedulers(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity} {
		for _, accountType := range []string{AccountTypeOAuth, AccountTypeAPIKey} {
			t.Run(platform+"/"+accountType, func(t *testing.T) {
				account := &Account{ID: 11, Platform: platform, Type: accountType, Status: StatusActive, Schedulable: true}
				ctx := WithUserAccountRestrictions(context.Background(), UserAccountRestrictions{1: {11}})
				group1, group2 := int64(1), int64(2)
				denied := withAccountRestrictionGroup(ctx, &group1)
				require.False(t, account.IsSchedulableForModelWithContext(denied, ""))
				require.True(t, account.IsSchedulableForModelWithContext(withAccountRestrictionGroup(ctx, &group2), ""))
				require.True(t, account.IsSchedulableForModelWithContext(context.Background(), ""), "other users unaffected")
				other := *account
				other.ID = 12
				require.True(t, other.IsSchedulableForModelWithContext(denied, ""))
				if platform == PlatformOpenAI {
					scheduler := &defaultOpenAIAccountScheduler{}
					ok, reason := scheduler.isAccountRequestCompatibleReason(ctx, account, OpenAIAccountScheduleRequest{GroupID: &group1})
					require.False(t, ok)
					require.Equal(t, "user_account_restricted", reason)
					require.Equal(t, "user_account_restricted", openAICompatibleAccountEligibilityFailureReason(denied, account, platform, "", false, ""))
				}
			})
		}
	}
}

func TestUserAccountRestrictionShadowAndGroupOverride(t *testing.T) {
	parentID := int64(11)
	account := &Account{ID: 12, ParentAccountID: &parentID}
	ctx := WithUserAccountRestrictions(context.Background(), UserAccountRestrictions{1: {11}})
	ctx = context.WithValue(ctx, ctxkey.Group, &Group{ID: 1})
	require.False(t, userAccountAllowed(ctx, account), "shadow must not bypass a blocked parent")
	group2 := int64(2)
	require.True(t, userAccountAllowed(withAccountRestrictionGroup(ctx, &group2), account))
}

func TestAccountRestrictionAuthSnapshotIsolation(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{UserID: 3, User: &User{ID: 3, AccountRestrictions: UserAccountRestrictions{1: {11}}}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	key.User.AccountRestrictions[1][0] = 99
	first := svc.snapshotToAPIKey("key1", snapshot)
	require.Equal(t, []int64{11}, first.User.AccountRestrictions[1])
	first.User.AccountRestrictions[1][0] = 88
	second := svc.snapshotToAPIKey("key2", snapshot)
	require.Equal(t, []int64{11}, second.User.AccountRestrictions[1])
}
