package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekToolResponseBillable(t *testing.T) {
	tests := []struct {
		name     string
		tool     DeepSeekTool
		status   int
		body     string
		billable bool
	}{
		{"search result", DeepSeekToolSearch, http.StatusOK, `{"results":[{"title":"a"}]}`, true},
		{"search empty", DeepSeekToolSearch, http.StatusOK, `{"results":[]}`, false},
		{"search error", DeepSeekToolSearch, http.StatusBadRequest, `{"results":[{"title":"a"}]}`, false},
		{"fetch content", DeepSeekToolFetch, http.StatusOK, `{"content":"article"}`, true},
		{"fetch empty", DeepSeekToolFetch, http.StatusOK, `{"content":"  "}`, false},
		{"fetch invalid json", DeepSeekToolFetch, http.StatusOK, `{`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.billable, deepSeekToolResponseBillable(tc.tool, tc.status, []byte(tc.body)))
		})
	}
}

func TestForwardDeepSeekToolPreservesWireAndBillsOnlySuccessfulResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		tool     DeepSeekTool
		request  string
		response string
		status   int
		billable bool
	}{
		{"search result", DeepSeekToolSearch, `{"query":"news"}`, `{"results":[{"title":"news"}]}`, http.StatusOK, true},
		{"fetch empty", DeepSeekToolFetch, `{"url":"https://example.com"}`, `{"content":""}`, http.StatusOK, false},
		{"search client error", DeepSeekToolSearch, `{"query":"news"}`, `{"error":"bad query"}`, http.StatusBadRequest, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: tc.status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tc.response)),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			account := &Account{Platform: PlatformDeepseek, Type: AccountTypeAPIKey, Credentials: map[string]any{
				"api_key": "sk-test", "base_url": "https://compat.example/v4",
			}}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/"+string(tc.tool), bytes.NewBufferString(tc.request))
			result, err := svc.ForwardDeepSeekTool(context.Background(), c, account, tc.tool, []byte(tc.request))
			require.NoError(t, err)
			require.Equal(t, tc.status, recorder.Code)
			require.JSONEq(t, tc.response, recorder.Body.String())
			require.Equal(t, "https://compat.example/v4/tools/"+string(tc.tool), upstream.lastReq.URL.String())
			require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
			require.JSONEq(t, tc.request, string(upstream.lastBody))
			if tc.billable {
				require.NotNil(t, result)
				require.Equal(t, 1, result.WebSearchCalls)
			} else {
				require.Nil(t, result)
			}
		})
	}
}

func TestForwardDeepSeekToolNotFoundCanTryAnotherAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"endpoint unavailable"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{Platform: PlatformDeepseek, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "sk-test", "base_url": "https://compat.example",
	}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/search", strings.NewReader(`{"query":"news"}`))
	result, err := svc.ForwardDeepSeekTool(context.Background(), c, account, DeepSeekToolSearch, []byte(`{"query":"news"}`))
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusNotFound, failover.StatusCode)
	require.Equal(t, 0, recorder.Body.Len())
}
