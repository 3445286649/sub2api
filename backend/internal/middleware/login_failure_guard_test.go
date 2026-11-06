package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func serveLoginGuard(t *testing.T, enabled func(*gin.Context) bool, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LoginFailureGuard(nil, enabled))
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.POST("/api/v1/auth/login", ok)
	r.POST("/api/v1/auth/register", ok)
	r.POST("/api/v1/auth/passkey/login/begin", ok)
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "203.0.113.7:4321"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLoginGuardKeyGroupsIPv6ByNetwork(t *testing.T) {
	plain := loginGuardKey("203.0.113.7")
	if len(plain) != 64 {
		t.Fatalf("expected a 64 char hash, got %q", plain)
	}
	if mapped := loginGuardKey("::ffff:203.0.113.7"); mapped != plain {
		t.Fatalf("mapped IPv4 must share the IPv4 key: %q vs %q", mapped, plain)
	}
	a := loginGuardKey("2001:db8:1:2:aaaa:bbbb:cccc:dddd")
	b := loginGuardKey("2001:db8:1:2:1111:2222:3333:4444")
	if a == "" || a != b {
		t.Fatalf("hosts in one /64 must share a key: %q vs %q", a, b)
	}
	if c := loginGuardKey("2001:db8:1:3::1"); c == a {
		t.Fatal("different /64 networks must not share a key")
	}
	for _, value := range []string{"", "unknown", "fe80::1%eth0"} {
		if key := loginGuardKey(value); key != "" {
			t.Fatalf("%q must have no key, got %q", value, key)
		}
	}
}

func TestLoginFailureGuardSwitchOffPassesThrough(t *testing.T) {
	off := func(*gin.Context) bool { return false }
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/passkey/login/begin"} {
		if w := serveLoginGuard(t, off, http.MethodPost, path); w.Code != http.StatusOK {
			t.Fatalf("%s with the switch off must pass, got %d", path, w.Code)
		}
	}
}

func TestLoginFailureGuardFailsClosedWithoutRedis(t *testing.T) {
	on := func(*gin.Context) bool { return true }
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/passkey/login/begin"} {
		w := serveLoginGuard(t, on, http.MethodPost, path)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("%s without redis must be rejected with 429, got %d", path, w.Code)
		}
		if w.Header().Get("Retry-After") == "" {
			t.Fatalf("%s must carry Retry-After", path)
		}
	}
}

func TestLoginFailureGuardIgnoresOtherRoutes(t *testing.T) {
	on := func(*gin.Context) bool { return true }
	if w := serveLoginGuard(t, on, http.MethodPost, "/api/v1/auth/register"); w.Code != http.StatusOK {
		t.Fatalf("register is not a login endpoint and must pass, got %d", w.Code)
	}
}

func TestIsLoginCredentialFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"invalid credentials", infraerrors.Unauthorized("INVALID_CREDENTIALS", "bad"), true},
		{"totp code", infraerrors.BadRequest("TOTP_INVALID_CODE", "bad"), true},
		{"passkey", infraerrors.Unauthorized("PASSKEY_VERIFICATION_FAILED", "bad"), true},
		{"disabled account", infraerrors.Forbidden("USER_NOT_ACTIVE", "x"), false},
		{"service fault", infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "x"), false},
		{"success", nil, false},
	}
	for _, tc := range cases {
		r := gin.New()
		var got bool
		r.Use(func(c *gin.Context) {
			c.Next()
			got = isLoginCredentialFailure(c)
		})
		r.POST("/x", func(c *gin.Context) {
			if tc.err == nil {
				c.Status(http.StatusOK)
				return
			}
			response.ErrorFrom(c, tc.err)
		})
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if got != tc.want {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.want, got)
		}
	}
}
