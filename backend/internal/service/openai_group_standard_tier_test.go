package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGroupStandardTier_OverridesClientAndGroupAcceleration(t *testing.T) {
	svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
		ServiceTier: OpenAIFastTierMissing, Action: OpenAIFastPolicyActionForcePriority, Scope: BetaPolicyScopeAll,
	}}})
	for _, platform := range []string{PlatformOpenAI, PlatformComposite} {
		for flags := 0; flags < 8; flags++ {
			for _, requested := range []string{"", "default", "priority", "fast", "ultrafast", "flex", "auto", "scale"} {
				t.Run(fmt.Sprintf("%s/switches=%d/requested=%s", platform, flags, requested), func(t *testing.T) {
					account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
					group := &Group{ID: 7, Platform: platform, Status: StatusActive, Hydrated: true,
						ForceOpenAIFast: flags&1 != 0, ForceOpenAIUltrafast: flags&2 != 0, FreeOpenAIFast: flags&4 != 0,
						OpenAIModelTiers: map[string]string{"public-standard": "default"},
					}
					ctx := context.WithValue(context.Background(), ctxkey.Group, group)
					ctx = context.WithValue(ctx, ctxkey.Model, "mapped-upstream")
					ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, "public-standard")
					body := []byte(`{"type":"response.create","model":"mapped-upstream"}`)
					if requested != "" {
						body = []byte(fmt.Sprintf(`{"type":"response.create","model":"mapped-upstream","service_tier":%q}`, requested))
					}
					updated, err := svc.applyOpenAIFastPolicyToBody(ctx, account, "mapped-upstream", body)
					require.NoError(t, err)
					require.Equal(t, "default", gjson.GetBytes(updated, "service_tier").String())
					frame, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "mapped-upstream", body)
					require.NoError(t, err)
					require.Nil(t, blocked)
					require.Equal(t, "default", gjson.GetBytes(frame, "service_tier").String())
					result := &OpenAIForwardResult{Model: "mapped-upstream", ServiceTier: extractOpenAIServiceTierFromBody(updated), UpstreamResponseServiceTier: "ultrafast"}
					details, free, err := resolveOpenAIRecordServiceTier(&APIKey{GroupID: &group.ID, Group: group}, account, result, "public-standard")
					require.NoError(t, err)
					require.Equal(t, "standard", details.BillingTier)
					require.Equal(t, "default", details.GroupTier)
					require.Equal(t, "group_model", details.Source)
					require.False(t, free, "explicit Standard is not free acceleration")
				})
			}
		}
	}
}

func TestOpenAIGroupStandardTier_RespectsGlobalPolicy(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true,
		OpenAIModelTiers: map[string]string{"standard-model": "default"},
	})
	for _, action := range []string{BetaPolicyActionBlock, BetaPolicyActionFilter, OpenAIFastPolicyActionForcePriority} {
		t.Run(action, func(t *testing.T) {
			svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
				ServiceTier: OpenAIFastTierAny, Action: action, Scope: BetaPolicyScopeAll,
			}}})
			body := []byte(`{"type":"response.create","model":"standard-model","service_tier":"ultrafast"}`)
			_, err := svc.applyOpenAIFastPolicyToBody(ctx, account, "standard-model", body)
			var blocked *OpenAIFastBlockedError
			require.ErrorAs(t, err, &blocked)
			_, wsBlocked, wsErr := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "standard-model", body)
			require.NoError(t, wsErr)
			require.NotNil(t, wsBlocked)
			if action != BetaPolicyActionBlock {
				require.Contains(t, blocked.Message, "conflicts")
				require.Contains(t, wsBlocked.Message, "conflicts")
			}
		})
	}
}
