//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestService_TestAccountConnection_GrokUsesXAIResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          13,
		Name:        "grok-oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"model_mapping": map[string]any{
				"grok": "grok-4.3",
			},
		},
	}
	repo := &mockAccountRepoForGemini{
		accountsByID: map[int64]*Account{account.ID: account},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n",
		)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/13/test", nil)

	err := svc.TestAccountConnection(c, account.ID, "grok", "", AccountTestModeDefault)
	require.NoError(t, err)

	require.Equal(t, "https://api.x.ai/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer grok-access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "grok-4.3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotContains(t, rec.Body.String(), "claude")
	require.Contains(t, rec.Body.String(), `"model":"grok-4.3"`)
	require.Contains(t, rec.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_ProbeGrokProtocolsDetectsSupportedAndUnknownProtocols(t *testing.T) {
	account := &Account{
		ID:          806,
		Name:        "grok-probe",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "grok-api-key",
			"base_url": "https://grok-gateway.example.com/v1",
		},
	}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
		},
		{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"route not found"}}`)),
		},
		{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
		},
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

	result, err := svc.ProbeGrokProtocols(context.Background(), account, "grok-4.5-high", "hi")
	require.NoError(t, err)
	require.Equal(t, GrokUpstreamProtocolOpenAIChatCompletions, result.RecommendedProtocol)
	require.Len(t, result.Results, 3)

	require.Equal(t, GrokProbeProtocolOpenAIChatCompletions, result.Results[0].Protocol)
	require.Equal(t, GrokProbeStatusSupported, result.Results[0].Status)
	require.True(t, result.Results[0].Supported)
	require.Equal(t, "https://grok-gateway.example.com/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer grok-api-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "grok-4.5-high", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, float64(1), gjson.GetBytes(upstream.bodies[0], "max_tokens").Float())

	require.Equal(t, GrokProbeProtocolAnthropicMessages, result.Results[1].Protocol)
	require.Equal(t, GrokProbeStatusUnsupported, result.Results[1].Status)
	require.False(t, result.Results[1].Supported)
	require.Equal(t, "https://grok-gateway.example.com/v1/messages?beta=true", upstream.requests[1].URL.String())
	require.Equal(t, "grok-api-key", upstream.requests[1].Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", upstream.requests[1].Header.Get("anthropic-version"))

	require.Equal(t, GrokProbeProtocolOpenAIResponses, result.Results[2].Protocol)
	require.Equal(t, GrokProbeStatusUnknown, result.Results[2].Status)
	require.False(t, result.Results[2].Supported)
	require.Equal(t, "https://grok-gateway.example.com/v1/responses", upstream.requests[2].URL.String())
	require.Equal(t, "Bearer grok-api-key", upstream.requests[2].Header.Get("Authorization"))
	require.Equal(t, float64(1), gjson.GetBytes(upstream.bodies[2], "max_output_tokens").Float())
}

func TestAccountTestService_ProbeGrokProtocolsRejectsNonGrokAPIKey(t *testing.T) {
	svc := &AccountTestService{httpUpstream: &httpUpstreamRecorder{}, cfg: upstreamModelSyncTestConfig()}
	_, err := svc.ProbeGrokProtocols(context.Background(), &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, "", "")
	require.Error(t, err)
}

func TestAccountTestService_ProbeGrokProtocolsDefaultsToAccountModelMapping(t *testing.T) {
	account := &Account{
		ID:          807,
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "grok-api-key",
			"base_url": "https://grok-gateway.example.com/v1",
			"model_mapping": map[string]any{
				"grok-4.5-high": "grok-4.5-high",
			},
		},
	}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`))},
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`))},
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`))},
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

	_, err := svc.ProbeGrokProtocols(context.Background(), account, "", "hi")
	require.NoError(t, err)
	require.Equal(t, "grok-4.5-high", gjson.GetBytes(upstream.bodies[0], "model").String())
}

func TestAccountTestService_TestAccountConnection_GrokAPIKeyOpenAICompatible(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          805,
		Name:        "grok-openai-compatible",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                "grok-api-key",
			"base_url":               "https://grok-gateway.example.com/v1",
			"grok_upstream_protocol": "openai_chat_completions",
		},
	}
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
				"data: [DONE]\n\n",
		)),
	}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/805/test", nil)

	err := svc.TestAccountConnection(c, account.ID, "grok-4", "ping", AccountTestModeDefault)
	require.NoError(t, err)

	require.Equal(t, "https://grok-gateway.example.com/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer grok-api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "grok-4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Contains(t, rec.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_TestAccountConnection_GrokAPIKeyAnthropicCompatible(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          806,
		Name:        "grok-anthropic-compatible",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                "grok-api-key",
			"base_url":               "https://grok-gateway.example.com",
			"grok_upstream_protocol": "anthropic_messages",
		},
	}
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n" +
				"data: {\"type\":\"message_stop\"}\n\n",
		)),
	}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: upstreamModelSyncTestConfig()}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/806/test", nil)

	err := svc.TestAccountConnection(c, account.ID, "claude-sonnet-4-5", "", AccountTestModeDefault)
	require.NoError(t, err)

	require.Equal(t, "https://grok-gateway.example.com/v1/messages?beta=true", upstream.lastReq.URL.String())
	require.Equal(t, "grok-api-key", upstream.lastReq.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", upstream.lastReq.Header.Get("anthropic-version"))
	require.Equal(t, "claude-sonnet-4-5", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotContains(t, rec.Body.String(), "Unsupported Grok account type")
	require.Contains(t, rec.Body.String(), `"type":"test_complete"`)
}
