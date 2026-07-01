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

func shouldRecordAccountHealthForwardError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	skipMarkers := []string{
		"concurrency limit exceeded for account",
		"concurrency limit exceeded for user",
		"billing",
		"quota",
		"invalid request",
		"prompt too long",
		"context canceled",
		"context cancelled",
	}
	for _, marker := range skipMarkers {
		if strings.Contains(msg, marker) {
			return false
		}
	}
	recordMarkers := []string{
		"upstream temporarily unavailable",
		"upstream response failed",
		"upstream error",
		"do request failed",
		"bad gateway",
		"empty response",
		"connection reset",
		"connection refused",
		"timeout",
		"deadline exceeded",
		"eof",
	}
	for _, marker := range recordMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func accountHealthForwardErrorCategory(err error) string {
	if err == nil {
		return "forward_error"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "timeout"
	case strings.Contains(msg, "upstream temporarily unavailable"):
		return "upstream_5xx"
	case strings.Contains(msg, "bad gateway"), strings.Contains(msg, "upstream error"), strings.Contains(msg, "do request failed"):
		return "upstream_5xx"
	default:
		return "forward_error"
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
