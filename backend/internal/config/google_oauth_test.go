package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoogleOAuthRedirectURLs(t *testing.T) {
	primary := "https://codeplan.codes" + GoogleOAuthCallbackPath
	secondary := "https://subapi.loucer.cn" + GoogleOAuthCallbackPath
	t.Run("single domain remains compatible", func(t *testing.T) {
		urls, err := GoogleOAuthRedirectURLs(EmailOAuthProviderConfig{RedirectURL: "http://localhost/custom", FrontendRedirectURL: "https://frontend.example/callback"})
		require.NoError(t, err)
		require.Empty(t, urls)
	})
	t.Run("multiline and explicit HTTPS port", func(t *testing.T) {
		urls, err := GoogleOAuthRedirectURLs(EmailOAuthProviderConfig{RedirectURL: primary, AdditionalRedirectURLs: "\r\n " + strings.Replace(secondary, ".cn/", ".cn:443/", 1) + " \r\n"})
		require.NoError(t, err)
		require.Len(t, urls, 2)
		require.Equal(t, "subapi.loucer.cn:443", urls[1].Host)
	})
	for _, raw := range []string{"https://%zz", "https://", "//subapi.loucer.cn" + GoogleOAuthCallbackPath, secondary + "?", strings.Repeat("x", 16385)} {
		t.Run("invalid_"+raw[:min(len(raw), 40)], func(t *testing.T) {
			_, err := GoogleOAuthRedirectURLs(EmailOAuthProviderConfig{RedirectURL: primary, AdditionalRedirectURLs: raw})
			require.Error(t, err)
		})
	}
	t.Run("empty primary is rejected", func(t *testing.T) {
		_, err := GoogleOAuthRedirectURLs(EmailOAuthProviderConfig{AdditionalRedirectURLs: secondary})
		require.Error(t, err)
	})
}
