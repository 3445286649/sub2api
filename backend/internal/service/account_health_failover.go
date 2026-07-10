package service

import (
	"context"
	"errors"
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
	if errors.Is(err, context.Canceled) {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	if accountHealthMessageIsNonAccountFault(msg) || accountHealthForwardErrorIsHTTP400(msg) {
		return false
	}
	skipMarkers := []string{
		"invalid request",
		"prompt too long",
		"context canceled",
		"context cancelled",
		"request canceled",
		"request cancelled",
		"client disconnected",
		"concurrency limit exceeded",
		"too many pending requests",
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
		"quota exceeded",
		"insufficient balance",
		"insufficient quota",
		"余额不足",
		"timeout",
		"deadline exceeded",
	}
	for _, marker := range recordMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	if accountHealthForwardErrorIsNetwork(msg) {
		return true
	}
	return false
}

func shouldRecordAccountHealthFailure(err *UpstreamFailoverError) bool {
	if err == nil || err.StatusCode == http.StatusBadRequest {
		return false
	}
	return !accountHealthMessageIsNonAccountFault(string(err.ResponseBody))
}

func accountHealthContextCanceled(ctx context.Context) bool {
	return ctx != nil && errors.Is(ctx.Err(), context.Canceled)
}

func accountHealthMessageIsNonAccountFault(message string) bool {
	msg := strings.ToLower(strings.TrimSpace(message))
	if msg == "" {
		return false
	}
	markers := []string{
		"unsupported parameter",
		"input must be a list",
		"invalid request",
		"invalid_request",
		"model_not_found",
		"model not found",
		"model does not exist",
		"model unavailable",
		"unknown model",
		"unsupported model",
		"model is not supported",
		"model mapping",
		"mapping does not support",
	}
	for _, marker := range markers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func accountHealthForwardErrorIsHTTP400(msg string) bool {
	markers := []string{
		"upstream error: 400",
		"upstream returned 400",
		"upstream returned error 400",
		"returned status 400",
		"status code 400",
		"status=400",
		"status_code=400",
		`"status":400`,
		`"status_code":400`,
		"http 400",
	}
	for _, marker := range markers {
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
	case strings.Contains(msg, "quota exceeded"), strings.Contains(msg, "insufficient balance"), strings.Contains(msg, "insufficient quota"), strings.Contains(msg, "余额不足"):
		return "quota_exceeded"
	case strings.Contains(msg, "model_not_found"), strings.Contains(msg, "model not found"), strings.Contains(msg, "model does not exist"), strings.Contains(msg, "model unavailable"), strings.Contains(msg, "unknown model"), strings.Contains(msg, "unsupported model"):
		return "model_not_found"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"):
		return "timeout"
	case accountHealthForwardErrorIsNetwork(msg):
		return "network_error"
	case strings.Contains(msg, "empty response"):
		return "empty_response"
	case strings.Contains(msg, "upstream temporarily unavailable"):
		return "upstream_5xx"
	case strings.Contains(msg, "bad gateway"), strings.Contains(msg, "upstream error"), strings.Contains(msg, "do request failed"):
		return "upstream_5xx"
	default:
		return "forward_error"
	}
}

func accountHealthForwardErrorIsNetwork(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		msg == "eof" ||
		strings.Contains(msg, "unexpected eof") ||
		strings.HasSuffix(msg, ": eof") ||
		strings.HasSuffix(msg, " eof") ||
		strings.Contains(msg, " eof ")
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
	case strings.Contains(normalized, "quota exceeded"),
		strings.Contains(normalized, "insufficient balance"),
		strings.Contains(normalized, "insufficient quota"),
		strings.Contains(normalized, "余额不足"):
		return "quota_exceeded"
	case strings.Contains(normalized, "model_not_found"),
		strings.Contains(normalized, "model not found"),
		strings.Contains(normalized, "model does not exist"),
		strings.Contains(normalized, "model unavailable"),
		strings.Contains(normalized, "unknown model"),
		strings.Contains(normalized, "unsupported model"):
		return "model_not_found"
	default:
		return "probe_failed"
	}
}

func AccountHealthProbeFailureCategory(message string) string {
	return accountHealthProbeFailureCategory(message)
}
