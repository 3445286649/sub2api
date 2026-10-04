//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/authidentity"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGoogleMultiDomainTestHandler() *AuthHandler {
	h, _ := newOAuthCaptchaTestHandler(false)
	h.settingSvc = service.NewSettingService(&oauthCaptchaSettingRepo{values: googleMultiDomainTestSettings()}, &config.Config{})
	return h
}

func googleMultiDomainTestSettings() map[string]string {
	return map[string]string{
		service.SettingKeyGoogleOAuthEnabled:                "true",
		service.SettingKeyGoogleOAuthClientID:               "test-client",
		service.SettingKeyGoogleOAuthClientSecret:           "test-secret",
		service.SettingKeyGoogleOAuthRedirectURL:            "https://codeplan.codes/api/v1/auth/oauth/google/callback",
		service.SettingKeyGoogleOAuthAdditionalRedirectURLs: "https://subapi.loucer.cn/api/v1/auth/oauth/google/callback",
	}
}

func TestGoogleMultiDomainRejectsUnlistedHostAndIgnoresForwardedHost(t *testing.T) {
	h := newGoogleMultiDomainTestHandler()
	for _, host := range []string{"evil.example", "codeplan.codes.evil.example", "codeplan.codes:8080", "codeplan.codes."} {
		for _, endpoint := range []string{"start", "callback?code=test-code&state=test-state"} {
			t.Run(host+"/"+endpoint, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodGet, "https://"+host+"/api/v1/auth/oauth/google/"+endpoint, nil)
				c.Request.Header.Set("X-Forwarded-Host", "codeplan.codes")
				c.Request.Header.Set("Forwarded", "host=codeplan.codes;proto=https")
				if endpoint == "start" {
					h.GoogleOAuthStart(c)
				} else {
					h.GoogleOAuthCallback(c)
				}
				require.Equal(t, http.StatusBadRequest, w.Code)
				require.Contains(t, w.Body.String(), "OAUTH_DOMAIN_NOT_ALLOWED")
				require.Empty(t, w.Header().Get("Location"))
				require.Empty(t, w.Result().Cookies())
			})
		}
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "https://subapi.loucer.cn/api/v1/auth/oauth/google/start?redirect_uri=https://evil.example/callback", nil)
	c.Request.Header.Set("X-Forwarded-Host", "codeplan.codes")
	h.GoogleOAuthStart(c)
	require.Equal(t, http.StatusFound, w.Code)
	u, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https://subapi.loucer.cn/api/v1/auth/oauth/google/callback", u.Query().Get("redirect_uri"))
}

func TestGoogleMultiDomainCallbackRequiresStateAndProvider(t *testing.T) {
	h := newGoogleMultiDomainTestHandler()
	for _, tc := range []struct{ name, query, state, provider, want string }{
		{"missing_cookie", "code=test&state=valid", "", "", "invalid_state"},
		{"wrong_state", "code=test&state=valid", "wrong", "google", "invalid_state"},
		{"wrong_provider", "code=test&state=valid", "valid", "github", "invalid_state"},
		{"missing_code", "state=valid", "valid", "google", "missing_params"},
		{"cancelled", "error=access_denied", "valid", "google", "provider_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "https://subapi.loucer.cn/api/v1/auth/oauth/google/callback?"+tc.query, nil)
			if tc.state != "" {
				c.Request.AddCookie(encodedCookie(emailOAuthStateCookieName, tc.state))
			}
			if tc.provider != "" {
				c.Request.AddCookie(encodedCookie(emailOAuthProviderCookie, tc.provider))
			}
			h.GoogleOAuthCallback(c)
			require.Equal(t, http.StatusFound, w.Code)
			require.True(t, strings.HasPrefix(w.Header().Get("Location"), "/auth/oauth/callback#"))
			require.Equal(t, tc.want, parseOAuthRedirectFragment(t, w.Header().Get("Location")).Get("error"))
			requireCookieCleared(t, w, emailOAuthStateCookieName)
		})
	}
}

func TestGoogleMultiDomainEndToEndUsesSameIdentityAndRedirectURI(t *testing.T) {
	tokenForms := make(chan url.Values, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", 400)
				return
			}
			tokenForms <- r.PostForm
			fmt.Fprint(w, `{"access_token":"upstream-test-token","token_type":"Bearer"}`)
		case "/userinfo":
			fmt.Fprint(w, `{"sub":"google-shared-subject","email":"existing@example.com","email_verified":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	h, db := newOAuthPendingFlowTestHandler(t, false)
	h.settingSvc = service.NewSettingService(&oauthCaptchaSettingRepo{values: googleMultiDomainTestSettings()}, &config.Config{
		GoogleOAuth: config.EmailOAuthProviderConfig{TokenURL: upstream.URL + "/token", UserInfoURL: upstream.URL + "/userinfo"},
	})
	user, err := db.User.Create().SetEmail("existing@example.com").SetUsername("existing").SetPasswordHash("hash").SetRole(service.RoleUser).SetStatus(service.StatusActive).Save(context.Background())
	require.NoError(t, err)
	for _, host := range []string{"codeplan.codes", "subapi.loucer.cn"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "https://"+host+"/api/v1/auth/oauth/google/start", nil)
		h.GoogleOAuthStart(c)
		require.Equal(t, http.StatusFound, w.Code)
		authURL, err := url.Parse(w.Header().Get("Location"))
		require.NoError(t, err)
		callbackURI := authURL.Query().Get("redirect_uri")
		callback := httptest.NewRecorder()
		cc, _ := gin.CreateTestContext(callback)
		cc.Request = httptest.NewRequest(http.MethodGet, callbackURI+"?code=test-code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
		for _, cookie := range w.Result().Cookies() {
			cc.Request.AddCookie(cookie)
		}
		h.GoogleOAuthCallback(cc)
		require.Equal(t, http.StatusFound, callback.Code)
		select {
		case form := <-tokenForms:
			require.Equal(t, callbackURI, form.Get("redirect_uri"))
			require.Equal(t, "test-code", form.Get("code"))
		case <-time.After(time.Second):
			t.Fatal("callback did not exchange the authorization code")
		}
		require.True(t, strings.HasPrefix(callback.Header().Get("Location"), "/auth/oauth/callback#"))
		require.Contains(t, callback.Header().Get("Location"), "access_token=")
		requireCookieCleared(t, callback, emailOAuthStateCookieName)
	}
	identity, err := db.AuthIdentity.Query().Where(authidentity.ProviderTypeEQ("google"), authidentity.ProviderSubjectEQ("google-shared-subject")).Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, user.ID, identity.UserID)
	count, err := db.User.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestGoogleMultiDomainSingleDomainAndGitHubCompatibility(t *testing.T) {
	for _, provider := range []string{"google", "github"} {
		t.Run(provider, func(t *testing.T) {
			h, _ := newOAuthCaptchaTestHandler(false)
			values := map[string]string{
				provider + "_oauth_enabled": "true", provider + "_oauth_client_id": "test", provider + "_oauth_client_secret": "test",
				provider + "_oauth_redirect_url": "https://legacy.example/custom-callback",
			}
			h.settingSvc = service.NewSettingService(&oauthCaptchaSettingRepo{values: values}, &config.Config{})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "https://other.example/start", nil)
			if provider == "google" {
				h.GoogleOAuthStart(c)
			} else {
				h.GitHubOAuthStart(c)
			}
			require.Equal(t, http.StatusFound, w.Code)
			u, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			require.Equal(t, "https://legacy.example/custom-callback", u.Query().Get("redirect_uri"))
		})
	}
}

func TestGoogleMultiDomainStartUsesCurrentHost(t *testing.T) {
	h := newGoogleMultiDomainTestHandler()
	for _, host := range []string{"codeplan.codes", "subapi.loucer.cn"} {
		t.Run(host, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "https://"+host+"/api/v1/auth/oauth/google/start", nil)
			h.GoogleOAuthStart(c)
			require.Equal(t, http.StatusFound, w.Code)
			location, err := url.Parse(w.Header().Get("Location"))
			require.NoError(t, err)
			require.Equal(t, "https://"+host+"/api/v1/auth/oauth/google/callback", location.Query().Get("redirect_uri"))
			state := findCookie(w.Result().Cookies(), emailOAuthStateCookieName)
			require.NotNil(t, state)
			require.Empty(t, state.Domain)
			require.True(t, state.HttpOnly)
			require.True(t, state.Secure)
			require.Equal(t, http.SameSiteLaxMode, state.SameSite)
			require.Equal(t, location.Query().Get("state"), decodeCookieValueForTest(t, state.Value))
		})
	}
}

func TestGoogleMultiDomainPostStartAndInvalidStoredConfig(t *testing.T) {
	h := newGoogleMultiDomainTestHandler()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "https://subapi.loucer.cn/api/v1/auth/oauth/google/start", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.GoogleOAuthStart(c)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data struct {
			AuthorizeURL string `json:"authorize_url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	u, err := url.Parse(body.Data.AuthorizeURL)
	require.NoError(t, err)
	require.Equal(t, "https://subapi.loucer.cn/api/v1/auth/oauth/google/callback", u.Query().Get("redirect_uri"))

	values := googleMultiDomainTestSettings()
	values[service.SettingKeyGoogleOAuthAdditionalRedirectURLs] = "http://unsafe.example/callback"
	h.settingSvc = service.NewSettingService(&oauthCaptchaSettingRepo{values: values}, &config.Config{})
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "https://codeplan.codes/api/v1/auth/oauth/google/start", nil)
	h.GoogleOAuthStart(c)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "OAUTH_CONFIG_INVALID")
	require.Empty(t, w.Header().Get("Location"))
	require.Empty(t, w.Result().Cookies())
}
