package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateDeepSeekToolRequest(t *testing.T) {
	tests := []struct {
		name  string
		tool  service.DeepSeekTool
		body  string
		valid bool
	}{
		{"search", service.DeepSeekToolSearch, `{"query":"hello","count":50,"need_content":true}`, true},
		{"empty query", service.DeepSeekToolSearch, `{"query":"  "}`, false},
		{"fractional count", service.DeepSeekToolSearch, `{"query":"hello","count":1.5}`, false},
		{"duplicate count", service.DeepSeekToolSearch, `{"query":"hello","count":1,"count":51}`, false},
		{"count too large", service.DeepSeekToolSearch, `{"query":"hello","count":51}`, false},
		{"fetch", service.DeepSeekToolFetch, `{"url":"https://example.com/path"}`, true},
		{"fetch userinfo", service.DeepSeekToolFetch, `{"url":"https://user:pass@example.com/"}`, false},
		{"duplicate url", service.DeepSeekToolFetch, `{"url":"https://example.com","url":"file:///etc/passwd"}`, false},
		{"fetch unsupported scheme", service.DeepSeekToolFetch, `{"url":"file:///etc/passwd"}`, false},
		{"malformed json", service.DeepSeekToolFetch, `{`, false},
		{"body too large", service.DeepSeekToolSearch, `{"query":"` + strings.Repeat("a", 1<<20) + `"}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDeepSeekToolRequest(tc.tool, []byte(tc.body))
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestDeepSeekToolRejectsDisabledGroupBeforeForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tool := range []service.DeepSeekTool{service.DeepSeekToolSearch, service.DeepSeekToolFetch} {
		t.Run(string(tool), func(t *testing.T) {
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/tools/"+string(tool), strings.NewReader(`{}`))
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: service.PlatformDeepseek}})
			(&OpenAIGatewayHandler{}).deepSeekTool(c, tool)
			require.Equal(t, http.StatusNotFound, response.Code)
		})
	}
}
