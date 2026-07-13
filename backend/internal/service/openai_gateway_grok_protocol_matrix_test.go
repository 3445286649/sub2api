//go:build unit

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

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func grokAnthropicProtocolTestAccount() *Account {
	return &Account{
		ID:          9201,
		Name:        "grok-anthropic-matrix",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 2,
		Credentials: map[string]any{
			"api_key":                "grok-test-key",
			"base_url":               "https://grok-vendor.example/v1",
			"grok_upstream_protocol": GrokUpstreamProtocolAnthropicMessages,
		},
	}
}

func anthropicMatrixSSE(inputTokens, outputTokens, cacheRead, cacheCreation int) string {
	return strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_matrix","type":"message","role":"assistant","content":[],"model":"grok-upstream","usage":{"input_tokens":` + fmt.Sprint(inputTokens) + `,"cache_read_input_tokens":` + fmt.Sprint(cacheRead) + `,"cache_creation_input_tokens":` + fmt.Sprint(cacheCreation) + `}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":` + fmt.Sprint(outputTokens) + `}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")
}

func TestGrokProtocolMatrix_ResponsesToAnthropicNonStreamingPreservesToolsAndCacheUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok-client","input":"weather","stream":false,"tools":[{"type":"function","name":"weather","description":"lookup","parameters":{"type":"object","properties":{}}}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(anthropicMatrixSSE(12, 7, 5, 3))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: rawChatCompletionsTestConfig()}

	result, err := svc.Forward(context.Background(), c, grokAnthropicProtocolTestAccount(), body)
	require.NoError(t, err)
	require.Equal(t, "https://grok-vendor.example/v1/messages?beta=true", upstream.lastReq.URL.String())
	require.True(t, urlvalidator.RequiresPublicIPValidation(upstream.lastReq.Context()))
	require.Equal(t, "weather", gjson.GetBytes(upstream.lastBody, "tools.0.name").String())
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.CacheReadInputTokens)
	require.Equal(t, 3, result.Usage.CacheCreationInputTokens)
	require.Equal(t, grokAnthropicMessagesEndpoint, result.UpstreamEndpoint)
	require.Equal(t, "ok", gjson.Get(recorder.Body.String(), "output.0.content.0.text").String())
}

func TestGrokProtocolMatrix_ChatToAnthropicStreamingPreservesCacheUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok-client","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true}}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(anthropicMatrixSSE(20, 8, 11, 4))),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: rawChatCompletionsTestConfig()}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, grokAnthropicProtocolTestAccount(), body, "", "")
	require.NoError(t, err)
	require.Equal(t, "https://grok-vendor.example/v1/messages?beta=true", upstream.lastReq.URL.String())
	require.Equal(t, 20, result.Usage.InputTokens)
	require.Equal(t, 8, result.Usage.OutputTokens)
	require.Equal(t, 11, result.Usage.CacheReadInputTokens)
	require.Equal(t, 4, result.Usage.CacheCreationInputTokens)
	require.Equal(t, grokAnthropicMessagesEndpoint, result.UpstreamEndpoint)
	require.Contains(t, recorder.Body.String(), `"content":"ok"`)
	require.Contains(t, recorder.Body.String(), `"cached_tokens":11`)
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
}

func TestGrokProtocolMatrix_ChatDefaultsToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok-4.5","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))

	account := &Account{ID: 9202, Name: "grok-responses-matrix", Platform: PlatformGrok, Type: AccountTypeAPIKey, Concurrency: 2, Extra: map[string]any{
		openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
	}, Credentials: map[string]any{
		"api_key": "grok-test-key", "base_url": "https://grok-vendor.example/v1",
	}}
	upstream := &httpUpstreamRecorder{resp: grokChatBridgeCompletedResponse("resp_matrix", 6)}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: rawChatCompletionsTestConfig()}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, GrokUpstreamProtocolOpenAIResponses, account.GetGrokUpstreamProtocol())
	require.Equal(t, "https://grok-vendor.example/v1/responses", upstream.lastReq.URL.String())
	require.True(t, urlvalidator.RequiresPublicIPValidation(upstream.lastReq.Context()))
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, 6, result.Usage.CacheReadInputTokens)
	require.Equal(t, "cached ok", gjson.Get(recorder.Body.String(), "choices.0.message.content").String())
}

func TestGrokProtocolMatrix_MessagesDefaultsToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok-4.5","max_tokens":32,"messages":[{"role":"user","content":"hi"}],"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	account := &Account{ID: 9203, Name: "grok-messages-responses-matrix", Platform: PlatformGrok, Type: AccountTypeAPIKey, Concurrency: 2, Credentials: map[string]any{
		"api_key": "grok-test-key", "base_url": "https://grok-vendor.example/v1",
	}}
	upstream := &httpUpstreamRecorder{resp: grokMessagesSSECompletedResponse("resp_messages_matrix", 4)}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: rawChatCompletionsTestConfig()}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, "https://grok-vendor.example/v1/responses", upstream.lastReq.URL.String())
	require.True(t, urlvalidator.RequiresPublicIPValidation(upstream.lastReq.Context()))
	require.Equal(t, 4, result.Usage.CacheReadInputTokens)
	require.Contains(t, recorder.Body.String(), `"type":"message"`)
	require.Equal(t, int64(4), gjson.Get(recorder.Body.String(), "usage.cache_read_input_tokens").Int())
}
