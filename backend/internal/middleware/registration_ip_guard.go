package middleware

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/authguard"
	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/gin-gonic/gin"
)

// RegistrationIPGuard marks public signup requests with the trusted client IP so
// the user repository can claim the lifetime per-IP quota in the same transaction
// that creates the account. It never rejects requests itself, and admin or CLI
// user creation carries no marker, so it is not counted.
//
// The address comes from Gin's server.trusted_proxies chain only. The legacy
// forwarded-header compatibility switch is deliberately ignored here, otherwise a
// client could pick its own quota bucket by sending a forged header.
func RegistrationIPGuard(enabled func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isRegistrationEntrypoint(c) {
			c.Next()
			return
		}
		if enabled == nil || enabled(c) {
			address := ippkg.GetTrustedClientIP(c)
			c.Request = c.Request.WithContext(authguard.WithRegistrationIP(c.Request.Context(), address))
		}
		c.Next()
	}
}

// isRegistrationEntrypoint covers every public path that can create a user:
// email registration and all OAuth entry, callback and completion routes. OAuth
// payment routes never create accounts and are left out.
func isRegistrationEntrypoint(c *gin.Context) bool {
	path := c.FullPath()
	return strings.HasSuffix(path, "/register") ||
		(strings.Contains(path, "/oauth/") && !strings.Contains(path, "/payment/"))
}
