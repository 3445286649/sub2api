package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	loginFailureLimit  = 10
	loginFailureWindow = 15 * time.Minute
	loginBanDuration   = time.Hour
)

// check and failure share one atomic script. A ban has a fixed expiry; blocked
// requests neither extend it nor reach password hashing. A ban key without a TTL
// is repaired to a full ban duration as a fail-safe.
var loginFailureScript = redis.NewScript(`
local banned = redis.call('PTTL', KEYS[2])
if banned == -1 then
  redis.call('PEXPIRE', KEYS[2], ARGV[3])
  return tonumber(ARGV[3])
end
if banned > 0 then return banned end
if ARGV[1] == 'check' then return 0 end
local failures = redis.call('INCR', KEYS[1])
if failures == 1 or redis.call('PTTL', KEYS[1]) == -1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
if failures >= tonumber(ARGV[4]) then
  redis.call('SET', KEYS[2], '1', 'PX', ARGV[3])
  redis.call('DEL', KEYS[1])
  return tonumber(ARGV[3])
end
return 0
`)

// LoginFailureGuard bans a client IP for an hour after 10 credential failures in
// 15 minutes on password, 2FA, passkey and OAuth bind-login endpoints. The switch is
// read on every request; when it is off the guard neither checks nor records.
// The address comes from Gin's server.trusted_proxies chain only. If Redis is
// unavailable or the address cannot be parsed, protected endpoints reject with 429
// (consistent with the existing fail-close route limits).
func LoginFailureGuard(client *redis.Client, enabled func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		attempt := isLoginCredentialAttempt(c)
		if !attempt && !isPasskeyLoginBegin(c) {
			c.Next()
			return
		}
		if enabled != nil && !enabled(c) {
			c.Next()
			return
		}
		key := loginGuardKey(ippkg.GetTrustedClientIP(c))
		if key == "" || client == nil {
			abortRateLimit(c, time.Minute)
			return
		}
		ttl, err := runLoginGuard(c.Request.Context(), client, key, "check")
		if err != nil {
			slog.Error("login failure guard unavailable")
			abortRateLimit(c, time.Minute)
			return
		}
		if ttl > 0 {
			abortRateLimit(c, time.Duration(ttl)*time.Millisecond)
			return
		}

		c.Next()
		if !attempt || !isLoginCredentialFailure(c) {
			return
		}
		// Failures are kept across successful logins so a known valid credential cannot
		// reset an attacker's spray. Client cancellation must not drop an evaluated failure.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 2*time.Second)
		defer cancel()
		if _, err := runLoginGuard(ctx, client, key, "failure"); err != nil {
			slog.Error("login failure guard could not record failure")
		}
	}
}

func isLoginCredentialAttempt(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost {
		return false
	}
	path := c.FullPath()
	return strings.HasSuffix(path, "/login") ||
		strings.HasSuffix(path, "/login/2fa") ||
		strings.HasSuffix(path, "/bind-login") ||
		strings.HasSuffix(path, "/passkey/login/finish")
}

func isPasskeyLoginBegin(c *gin.Context) bool {
	return c.Request.Method == http.MethodPost && strings.HasSuffix(c.FullPath(), "/passkey/login/begin")
}

// isLoginCredentialFailure counts only reasons the server itself selected for a
// wrong credential. Malformed requests, disabled accounts, expired sessions,
// rate-limit 429s and 5xx faults are not credential failures.
func isLoginCredentialFailure(c *gin.Context) bool {
	status := c.Writer.Status()
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError || status == http.StatusTooManyRequests {
		return false
	}
	switch response.ErrorReason(c) {
	case "INVALID_CREDENTIALS", "TOTP_INVALID_CODE", "PASSKEY_VERIFICATION_FAILED":
		return true
	default:
		return false
	}
}

func runLoginGuard(ctx context.Context, client *redis.Client, key, action string) (int64, error) {
	// Hash tags keep both keys in one slot if Redis Cluster is adopted.
	prefix := "login_guard:{" + key + "}:"
	return loginFailureScript.Run(ctx, client, []string{prefix + "failures", prefix + "banned"},
		action, loginFailureWindow.Milliseconds(), loginBanDuration.Milliseconds(), loginFailureLimit).Int64()
}

// loginGuardKey hashes an IPv4 address, or an IPv6 /64 network, so rotating host
// bits does not dodge the ban. An unparsable address has no key.
func loginGuardKey(address string) string {
	ip, err := netip.ParseAddr(strings.TrimSpace(address))
	if err != nil || ip.Zone() != "" {
		return ""
	}
	ip = ip.Unmap()
	origin := ip.String()
	if ip.Is6() {
		origin = netip.PrefixFrom(ip, 64).Masked().String()
	}
	sum := sha256.Sum256([]byte(origin))
	return hex.EncodeToString(sum[:])
}
