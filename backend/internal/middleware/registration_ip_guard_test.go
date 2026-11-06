package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/authguard"
	"github.com/gin-gonic/gin"
)

const registrationGuardTestAddr = "203.0.113.7:4321"

func serveRegistrationGuard(t *testing.T, enabled func(*gin.Context) bool, method, path string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	seen := "unset"
	r := gin.New()
	r.Use(RegistrationIPGuard(enabled))
	handler := func(c *gin.Context) {
		if key, ok := authguard.RegistrationIP(c.Request.Context()); ok {
			seen = key
		} else {
			seen = "none"
		}
		c.Status(http.StatusOK)
	}
	r.POST("/api/v1/auth/register", handler)
	r.POST("/api/v1/auth/login", handler)
	r.GET("/api/v1/auth/oauth/github/callback", handler)
	r.GET("/api/v1/auth/oauth/wechat/payment/callback", handler)

	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = registrationGuardTestAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("guard must never reject requests, got %d", w.Code)
	}
	return seen
}

func TestRegistrationIPGuardMarksRegisterAndOAuthRoutes(t *testing.T) {
	want := authguard.IPKey("203.0.113.7")
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/auth/register"},
		{http.MethodGet, "/api/v1/auth/oauth/github/callback"},
	} {
		if got := serveRegistrationGuard(t, func(*gin.Context) bool { return true }, tc.method, tc.path); got != want {
			t.Fatalf("%s %s: expected key %q, got %q", tc.method, tc.path, want, got)
		}
		if got := serveRegistrationGuard(t, nil, tc.method, tc.path); got != want {
			t.Fatalf("%s %s with nil switch: expected key %q, got %q", tc.method, tc.path, want, got)
		}
	}
}

func TestRegistrationIPGuardLeavesOtherRoutesUnmarked(t *testing.T) {
	on := func(*gin.Context) bool { return true }
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodGet, "/api/v1/auth/oauth/wechat/payment/callback"},
	} {
		if got := serveRegistrationGuard(t, on, tc.method, tc.path); got != "none" {
			t.Fatalf("%s %s must not be marked, got %q", tc.method, tc.path, got)
		}
	}
}

func TestRegistrationIPGuardDoesNothingWhenSwitchIsOff(t *testing.T) {
	off := func(*gin.Context) bool { return false }
	if got := serveRegistrationGuard(t, off, http.MethodPost, "/api/v1/auth/register"); got != "none" {
		t.Fatalf("a disabled switch must not mark the request, got %q", got)
	}
}
