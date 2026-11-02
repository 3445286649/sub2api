package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Compatible upstreams can return complete JSON with an SSE MIME type.
// The billed usage must match the JSON delivered to the client.
func TestForward_NonStreamingJSONWithSSEContentTypeRetainsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, contentType := range []string{"application/json", "text/event-stream"} {
			t.Run(fmt.Sprintf("passthrough=%v/type=%s", passthrough, contentType), func(t *testing.T) {
				repo := &openAIRecordUsageLogRepoStub{inserted: true}
				svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				svc.settingService = NewSettingService(&openAIFastPolicyRepoStub{values: map[string]string{}}, svc.cfg)
				response := `{"id":"resp_mislabeled","object":"response","status":"completed","model":"gpt-6-astra","service_tier":"default","output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":4234,"input_tokens_details":{"cached_tokens":4096},"output_tokens":5,"total_tokens":4239}}`
				svc.httpUpstream = &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(response))}}
				group := &Group{ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RateMultiplier: 1, UsageAccelerationBadge: "fast"}
				ctx := context.WithValue(context.Background(), ctxkey.Group, group)
				body := []byte(`{"model":"gpt-6-astra","input":"OK","stream":false,"service_tier":"ultrafast"}`)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
				account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}, Extra: map[string]any{"openai_responses_supported": true, "openai_passthrough": passthrough}, Concurrency: 1}
				result, err := svc.Forward(ctx, c, account, body)
				require.NoError(t, err)
				require.Equal(t, 4234, result.Usage.InputTokens)
				require.Equal(t, 4096, result.Usage.CacheReadInputTokens)
				require.Equal(t, 5, result.Usage.OutputTokens)
				require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
				key := &APIKey{ID: 101, GroupID: &group.ID, Group: group}
				require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, User: &User{ID: 201}, Account: account}))
				require.Equal(t, 138, repo.lastLog.InputTokens)
				require.Equal(t, 4096, repo.lastLog.CacheReadTokens)
				require.Equal(t, 5, repo.lastLog.OutputTokens)
				require.Greater(t, repo.lastLog.ActualCost, 0.0)
				require.Equal(t, "standard", repo.lastLog.ServiceTierDetails.BillingTier)
			})
		}
	}
}
