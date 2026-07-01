package service

import (
	"net/http"
	"strings"
)

func accountHealthFailureCategory(statusCode int) string {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return "rate_limited"
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return "auth_error"
	case statusCode >= 500:
		return "upstream_5xx"
	case statusCode >= 400:
		return "upstream_4xx"
	case statusCode == 0:
		return "network_error"
	default:
		return "upstream_error"
	}
}

func accountHealthProbeFailureCategory(message string) string {
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "invalid_api_key"),
		strings.Contains(normalized, "invalid api key"),
		strings.Contains(normalized, "unauthorized"),
		strings.Contains(normalized, "authentication failed"),
		strings.Contains(normalized, "returned 401"),
		strings.Contains(normalized, "returned 403"):
		return "auth_error"
	default:
		return "probe_failed"
	}
}

func AccountHealthProbeFailureCategory(message string) string {
	return accountHealthProbeFailureCategory(message)
}
