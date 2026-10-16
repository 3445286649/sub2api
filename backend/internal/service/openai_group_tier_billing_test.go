package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayServiceRecordUsage_GroupTierIsBillingAuthority(t *testing.T) {
	for _, tier := range []string{OpenAIFastTierPriority, OpenAIFastTierUltrafast} {
		for _, observed := range []string{"fast", "ultrafast", "default", ""} {
			for _, free := range []bool{false, true} {
				t.Run(tier+"/"+observed+"/free="+map[bool]string{true: "yes", false: "no"}[free], func(t *testing.T) {
					repo := &openAIRecordUsageLogRepoStub{inserted: true}
					svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
					id := int64(77)
					key := &APIKey{ID: 101, GroupID: &id, Group: &Group{
						ID: id, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RateMultiplier: 1,
						ForceOpenAIFast: true, ForceOpenAIUltrafast: true, FreeOpenAIFast: free,
						OpenAIModelTiers: map[string]string{"gpt-6-astra": tier},
					}}
					tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}
					err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
						Result: &OpenAIForwardResult{RequestID: "group-tier", Model: "gpt-6-astra", ServiceTier: &tier,
							UpstreamResponseServiceTier: observed, Duration: time.Second,
							Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: 500}},
						APIKey: key, User: &User{ID: 201}, Account: &Account{ID: 301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
					})
					require.NoError(t, err)
					configured, err := svc.billingService.CalculateCostWithServiceTier("gpt-6-astra", tokens, 1, tier)
					require.NoError(t, err)
					customer := configured
					if free {
						customer, err = svc.billingService.CalculateCostWithServiceTier("gpt-6-astra", tokens, 1, "")
						require.NoError(t, err)
					}
					require.Equal(t, tier, *repo.lastLog.ServiceTier)
					require.InDelta(t, configured.TotalCost, repo.lastLog.TotalCost, 1e-10)
					require.InDelta(t, customer.ActualCost, repo.lastLog.ActualCost, 1e-10)
				})
			}
		}
	}
}

func TestForward_GroupTierBillingSnapshot_HTTPAndSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%v", stream), func(t *testing.T) {
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			svc.settingService = NewSettingService(&openAIFastPolicyRepoStub{values: map[string]string{}}, svc.cfg)
			response := `{"id":"resp_group","object":"response","status":"completed","model":"gpt-6-astra","service_tier":"fast","output":[],"usage":{"input_tokens":1000,"output_tokens":500}}`
			contentType := "application/json"
			if stream {
				response = "data: {\"type\":\"response.completed\",\"response\":" + response + "}\n\ndata: [DONE]\n\n"
				contentType = "text/event-stream"
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(response))}}
			svc.httpUpstream = upstream
			group := &Group{ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RateMultiplier: 1,
				ForceOpenAIUltrafast: true, OpenAIModelTiers: map[string]string{"public-astra": "ultrafast"}}
			ctx := context.WithValue(context.Background(), ctxkey.Group, group)
			ctx = context.WithValue(ctx, ctxkey.Model, "public-astra")
			body := []byte(fmt.Sprintf(`{"model":"gpt-6-astra","input":"OK","stream":%v}`, stream))
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
			account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}, Extra: map[string]any{"openai_responses_supported": true}, Concurrency: 1}
			result, err := svc.Forward(ctx, c, account, body)
			require.NoError(t, err)
			require.Equal(t, "ultrafast", gjson.GetBytes(upstream.lastBody, "service_tier").String())
			require.Equal(t, "priority", result.UpstreamResponseServiceTier)
			require.Contains(t, rec.Body.String(), `"service_tier":"fast"`, "do not falsify upstream response")
			// Simulate a settings change while the usage task is queued.
			group.OpenAIModelTiers["public-astra"] = "priority"
			group.ForceOpenAIUltrafast = false
			group.FreeOpenAIFast = true
			key := &APIKey{ID: 101, GroupID: &group.ID, Group: group}
			require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, User: &User{ID: 201}, Account: account}))
			require.Equal(t, "ultrafast", repo.lastLog.ServiceTierDetails.BillingTier)
			require.Equal(t, "ultrafast", repo.lastLog.ServiceTierDetails.GroupTier)
			require.Equal(t, "priority", repo.lastLog.ServiceTierDetails.UpstreamTier)
			require.Equal(t, "group_model", repo.lastLog.ServiceTierDetails.Source)
			// A different group using the same upstream/model retains legacy downgrade behavior.
			other := &Group{ID: 78, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RateMultiplier: 1}
			key.Group, key.GroupID = other, &other.ID
			tier := "ultrafast"
			otherResult := &OpenAIForwardResult{Model: "gpt-6-astra", ServiceTier: &tier, UpstreamResponseServiceTier: "fast", Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: 500}}
			require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: otherResult, APIKey: key, User: &User{ID: 201}, Account: account}))
			require.Equal(t, "priority", repo.lastLog.ServiceTierDetails.BillingTier)
			require.Empty(t, repo.lastLog.ServiceTierDetails.GroupTier)
			require.ErrorContains(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, User: &User{ID: 201}, Account: account}), "does not match")
		})
	}
}

func TestOpenAIGroupTier_WSConflictsAndModelIsolation(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	group := &Group{ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, ForceOpenAIFast: true, ForceOpenAIUltrafast: true, OpenAIModelTiers: map[string]string{"gpt-6-astra": "ultrafast"}}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	for _, action := range []string{BetaPolicyActionFilter, OpenAIFastPolicyActionForcePriority} {
		svc := newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{ServiceTier: "ultrafast", Action: action, Scope: BetaPolicyScopeAll}}})
		_, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(ctx, account, "gpt-6-astra", []byte(`{"type":"response.create","model":"gpt-6-astra"}`))
		require.NoError(t, err)
		require.NotNil(t, blocked)
		require.Contains(t, blocked.Message, "conflicts")
	}
	svc := newOpenAIGatewayServiceWithSettings(t, DefaultOpenAIFastPolicySettings())
	for _, model := range []string{"gpt-6-astra", "gpt-6.1-sol", "gpt-6-astra"} {
		turnCtx, snapshot := freezeOpenAIGroupTier(context.WithValue(ctx, ctxkey.RequestedPublicModel, model), account, model)
		frame, blocked, err := svc.applyOpenAIFastPolicyToWSResponseCreate(turnCtx, account, model, []byte(`{"type":"response.create"}`))
		require.NoError(t, err)
		require.Nil(t, blocked)
		result := &OpenAIForwardResult{Model: model, ServiceTier: extractOpenAIServiceTierFromBody(frame), UpstreamResponseServiceTier: "default", groupTierSnapshot: snapshot}
		details, _, err := resolveOpenAIRecordServiceTier(&APIKey{GroupID: &group.ID, Group: group}, account, result, model)
		require.NoError(t, err)
		if model == "gpt-6-astra" {
			require.Equal(t, "ultrafast", details.BillingTier)
		} else {
			require.Equal(t, "priority", details.BillingTier)
		}
	}
}

func TestOpenAIGroupTier_WebSocketTurnsUseFrozenBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{OpenAIWSIngressModePassthrough, OpenAIWSIngressModeCtxPool} {
		t.Run(mode, func(t *testing.T) {
			group := &Group{ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RateMultiplier: 1, ForceOpenAIFast: true, ForceOpenAIUltrafast: true, OpenAIModelTiers: map[string]string{"gpt-6-astra": "ultrafast"}}
			freeGroup := *group
			freeGroup.FreeOpenAIFast = true
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxkey.Group, group))
			defer cancel()
			upstream := newStagedPassthroughConn()
			cfg := passthroughLifecycleConfig()
			cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
			cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 5
			cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
			cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
			svc := newPassthroughLifecycleService(cfg, upstream)
			account := passthroughLifecycleAccount()
			account.Extra["openai_apikey_responses_websockets_v2_mode"] = mode
			if mode == OpenAIWSIngressModeCtxPool {
				svc.openaiWSPool = newOpenAIWSConnPool(cfg)
				svc.openaiWSPool.setClientDialerForTest(&stagedPassthroughDialer{conn: &groupTierCaptureConn{upstream}})
			}
			results := make(chan *OpenAIForwardResult, 4)
			hooks := &OpenAIWSIngressHooks{
				InitialRequestModel: "gpt-6-astra",
				TierPolicyGroup: func(turn int) *Group {
					if turn == 3 {
						return &freeGroup
					}
					return group
				},
				AfterTurn: func(_ int, r *OpenAIForwardResult, e error) {
					if e == nil && r != nil {
						results <- r
					}
				},
			}
			server, _ := startPassthroughHookRecordingServer(t, ctx, svc, account, hooks)
			defer server.Close()
			client := dialPassthroughLifecycleClientWithPayload(t, server, `{"type":"response.create","model":"gpt-6-astra","input":[]}`)
			defer client.CloseNow()
			for i, model := range []string{"gpt-6-astra", "gpt-6.1-sol", "gpt-6-astra"} {
				if i > 0 {
					writeCtx, writeCancel := context.WithTimeout(ctx, 3*time.Second)
					err := client.Write(writeCtx, coderws.MessageText, []byte(fmt.Sprintf(`{"type":"response.create","model":%q,"input":[]}`, model)))
					writeCancel()
					require.NoError(t, err)
				}
				outbound := requirePassthroughUpstreamWrite(t, upstream, 3*time.Second)
				want := "ultrafast"
				if i == 1 {
					want = "priority"
				}
				require.Equal(t, want, gjson.GetBytes(outbound, "service_tier").String())
				upstream.Send(fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_tier_%d","model":%q,"service_tier":"fast","usage":{"input_tokens":1000,"output_tokens":500}}}`, i, model))
				event, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
				require.NoError(t, err)
				require.Contains(t, string(event), `"service_tier":"fast"`)
				var result *OpenAIForwardResult
				select {
				case result = <-results:
				case <-time.After(3 * time.Second):
					t.Fatal("missing per-turn result")
				}
				require.NotNil(t, result.groupTierSnapshot)
				require.Equal(t, want, result.groupTierSnapshot.Tier)
				repo := &openAIRecordUsageLogRepoStub{inserted: true}
				billing := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				keyGroup := group
				if i == 2 {
					keyGroup = &freeGroup
				}
				require.NoError(t, billing.RecordUsage(ctx, &OpenAIRecordUsageInput{Result: result, APIKey: &APIKey{ID: 1, GroupID: &group.ID, Group: keyGroup}, Account: account, User: &User{ID: 1}}))
				expectedCustomer := want
				if i == 2 {
					expectedCustomer = "standard"
				}
				require.Equal(t, expectedCustomer, repo.lastLog.ServiceTierDetails.BillingTier)
				if i == 2 {
					require.Equal(t, "free_fast", repo.lastLog.ServiceTierDetails.Source)
				}
			}
			require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
		})
	}
}

type groupTierCaptureConn struct{ *stagedPassthroughConn }

func (c *groupTierCaptureConn) WriteJSON(ctx context.Context, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.WriteFrame(ctx, coderws.MessageText, data)
}
