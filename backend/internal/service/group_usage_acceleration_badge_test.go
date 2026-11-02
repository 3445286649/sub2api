//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestUsageBadgeOnly_StandardRequestAndFrozenReceipt(t *testing.T) {
	for _, badge := range []string{"fast", "ultrafast", "free_fast", "billed_fast", "billed_ultrafast"} {
		for _, requested := range []string{"", "priority", "fast", "ultrafast", "auto", "flex"} {
			for _, action := range []string{BetaPolicyActionPass, BetaPolicyActionFilter, OpenAIFastPolicyActionForcePriority} {
				t.Run(fmt.Sprintf("%s/%s/%s", badge, requested, action), func(t *testing.T) {
					svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{ServiceTier: OpenAIFastTierAny, Action: action, Scope: BetaPolicyScopeAll}}})
					group := &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, UsageAccelerationBadge: badge}
					account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
					ctx := context.WithValue(context.Background(), ctxkey.Group, group)
					ctx, snapshot := freezeOpenAIGroupTier(ctx, account, "test-model")
					body := []byte(fmt.Sprintf(`{"type":"response.create","model":"test-model","service_tier":%q}`, requested))
					updated, err := svc.applyOpenAIFastPolicyToBody(ctx, account, "test-model", body)
					require.NoError(t, err)
					require.Equal(t, "default", gjson.GetBytes(updated, "service_tier").String())
					frame, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "test-model", body)
					require.NoError(t, err)
					require.Nil(t, blocked)
					require.Equal(t, "default", gjson.GetBytes(frame, "service_tier").String())
					group.UsageAccelerationBadge = ""
					group.ForceOpenAIFast = true
					result := &OpenAIForwardResult{Model: "test-model", ServiceTier: extractOpenAIServiceTierFromBody(updated), UpstreamResponseServiceTier: "priority"}
					stampOpenAIGroupTier(result, snapshot)
					details, free, err := resolveOpenAIRecordServiceTier(&APIKey{GroupID: &group.ID, Group: group}, account, result, "test-model")
					require.NoError(t, err)
					require.False(t, free)
					require.Equal(t, "standard", details.BillingTier)
					require.Equal(t, "group_display_standard", details.Source)
					require.Equal(t, badge, details.UsageAccelerationBadge)
				})
			}
		}
	}
}

func TestUsageBadge_RealTierAndFreeFastTakePrecedence(t *testing.T) {
	for _, tc := range []struct {
		tier              string
		fast, ultra, free bool
		want              string
		wantFree          bool
	}{
		{"", false, false, false, "default", false},
		{"ultrafast", false, true, false, "ultrafast", false},
		{"priority", true, false, false, "priority", false},
		{"priority", true, false, true, "priority", true},
		{"default", true, true, true, "default", false},
	} {
		t.Run(fmt.Sprintf("%+v", tc), func(t *testing.T) {
			group := &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, UsageAccelerationBadge: "ultrafast", ForceOpenAIFast: tc.fast, ForceOpenAIUltrafast: tc.ultra, FreeOpenAIFast: tc.free, OpenAIModelTiers: map[string]string{"test-model": tc.tier}}
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
			ctx := context.WithValue(context.Background(), ctxkey.Group, group)
			require.Equal(t, tc.want, openAIGroupForcedTier(ctx, account, "test-model"))
			result := &OpenAIForwardResult{Model: "test-model"}
			details, free, err := resolveOpenAIRecordServiceTier(&APIKey{GroupID: &group.ID, Group: group}, account, result, "test-model")
			require.NoError(t, err)
			require.Equal(t, tc.wantFree, free)
			require.Equal(t, customerServiceTier(tc.want), details.BillingTier)
		})
	}
}

func TestUsageBadge_AdminValidation(t *testing.T) {
	repo := &groupRepoStubForAdmin{}
	svc := &adminServiceImpl{groupRepo: repo}
	_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "bad", Platform: PlatformOpenAI, UsageAccelerationBadge: "invalid"})
	require.Error(t, err)
	require.Nil(t, repo.created)
	for _, badge := range []string{"", "fast", "ultrafast", "free_fast", "billed_fast", "billed_ultrafast"} {
		created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "valid", Platform: PlatformOpenAI, UsageAccelerationBadge: badge, RateMultiplier: 1})
		require.NoError(t, err)
		require.Equal(t, badge, created.UsageAccelerationBadge)
	}
	repo.getByID = &Group{ID: 7, Name: "existing", Platform: PlatformOpenAI, Status: StatusActive, UsageAccelerationBadge: "fast"}
	badge := "invalid"
	_, err = svc.UpdateGroup(context.Background(), 7, &UpdateGroupInput{UsageAccelerationBadge: &badge})
	require.Error(t, err)
	badge = ""
	updated, err := svc.UpdateGroup(context.Background(), 7, &UpdateGroupInput{UsageAccelerationBadge: &badge})
	require.NoError(t, err)
	require.Empty(t, updated.UsageAccelerationBadge)
}

func TestUsageBadge_DoesNotBypassGlobalBlock(t *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
		ServiceTier: OpenAIFastTierAny, Action: BetaPolicyActionBlock, Scope: BetaPolicyScopeAll,
	}}})
	group := &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, UsageAccelerationBadge: "fast"}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	body := []byte(`{"type":"response.create","model":"test-model","service_tier":"priority"}`)
	_, err := svc.applyOpenAIFastPolicyToBody(ctx, account, "test-model", body)
	var blocked *OpenAIFastBlockedError
	require.ErrorAs(t, err, &blocked)
	_, wsBlocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "test-model", body)
	require.NoError(t, err)
	require.NotNil(t, wsBlocked)
}
