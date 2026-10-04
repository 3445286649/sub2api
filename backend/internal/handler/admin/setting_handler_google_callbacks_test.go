//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

const googlePrimaryCallback = "https://codeplan.codes/api/v1/auth/oauth/google/callback"
const googleSecondaryCallback = "https://subapi.loucer.cn/api/v1/auth/oauth/google/callback"

func TestGoogleCallbacksSettingsAudit(t *testing.T) {
	changed := diffSettings(&service.SystemSettings{}, &service.SystemSettings{
		GoogleOAuthAdditionalRedirectURLs: googleSecondaryCallback,
	}, nil, nil, UpdateSettingsRequest{})
	require.Contains(t, changed, "google_oauth_additional_redirect_urls")
}

func TestGoogleCallbacksSettingsRoundTripAndPartialUpdates(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyGoogleOAuthRedirectURL:  googlePrimaryCallback,
		service.SettingKeyGoogleOAuthClientSecret: "stored-secret",
	})
	rec := doUpdateSettings(t, h, map[string]any{"google_oauth_additional_redirect_urls": googleSecondaryCallback}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, googleSecondaryCallback, body.Data["google_oauth_additional_redirect_urls"])
	require.Equal(t, googleSecondaryCallback, repo.values[service.SettingKeyGoogleOAuthAdditionalRedirectURLs])
	require.Equal(t, googlePrimaryCallback, repo.values[service.SettingKeyGoogleOAuthRedirectURL])
	require.Equal(t, "stored-secret", repo.values[service.SettingKeyGoogleOAuthClientSecret])

	rec = doUpdateSettings(t, h, map[string]any{"site_name": "updated"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, googleSecondaryCallback, repo.values[service.SettingKeyGoogleOAuthAdditionalRedirectURLs])

	rec = doUpdateSettings(t, h, map[string]any{"google_oauth_redirect_url": "https://evil.example/wrong-path"}, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, googlePrimaryCallback, repo.values[service.SettingKeyGoogleOAuthRedirectURL])

	rec = doUpdateSettings(t, h, map[string]any{"google_oauth_additional_redirect_urls": ""}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Empty(t, repo.values[service.SettingKeyGoogleOAuthAdditionalRedirectURLs])
}

func TestGoogleCallbacksSettingsRejectUnsafeURLs(t *testing.T) {
	for _, tc := range []struct{ name, additional, frontend string }{
		{"http", "http://subapi.loucer.cn/api/v1/auth/oauth/google/callback", "/auth/oauth/callback"},
		{"wildcard", "https://*.loucer.cn/api/v1/auth/oauth/google/callback", "/auth/oauth/callback"},
		{"userinfo", "https://user:pass@subapi.loucer.cn/api/v1/auth/oauth/google/callback", "/auth/oauth/callback"},
		{"query", googleSecondaryCallback + "?next=evil", "/auth/oauth/callback"},
		{"fragment", googleSecondaryCallback + "#fragment", "/auth/oauth/callback"},
		{"wrong_path", "https://subapi.loucer.cn/callback", "/auth/oauth/callback"},
		{"wrong_port", "https://subapi.loucer.cn:8080/api/v1/auth/oauth/google/callback", "/auth/oauth/callback"},
		{"duplicate", googlePrimaryCallback, "/auth/oauth/callback"},
		{"absolute_frontend", googleSecondaryCallback, "https://codeplan.codes/auth/oauth/callback"},
		{"protocol_relative", googleSecondaryCallback, "//evil.example"},
		{"backslash", googleSecondaryCallback, `/\evil.example`},
		{"encoded_backslash", googleSecondaryCallback, "/%5cevil.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyGoogleOAuthRedirectURL: googlePrimaryCallback})
			rec := doUpdateSettings(t, h, map[string]any{
				"google_oauth_additional_redirect_urls": tc.additional,
				"google_oauth_frontend_redirect_url":    tc.frontend,
			}, nil)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "INVALID_GOOGLE_OAUTH_CALLBACKS")
			require.Empty(t, repo.values[service.SettingKeyGoogleOAuthAdditionalRedirectURLs])
		})
	}
}
