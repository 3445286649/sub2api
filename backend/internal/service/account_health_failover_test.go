//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountHealthForwardErrorCategoryClassifiesNetworkErrors(t *testing.T) {
	cases := []string{
		"connection reset by peer",
		"connection refused",
		"unexpected EOF",
		"EOF",
	}

	for _, message := range cases {
		require.True(t, shouldRecordAccountHealthForwardError(errors.New(message)), message)
		require.Equal(t, "network_error", accountHealthForwardErrorCategory(errors.New(message)), message)
	}
}

func TestShouldRecordAccountHealthForwardErrorDoesNotMatchEmbeddedEOF(t *testing.T) {
	err := errors.New("geofencing policy blocked")

	require.False(t, shouldRecordAccountHealthForwardError(err))
	require.Equal(t, "forward_error", accountHealthForwardErrorCategory(err))
}

func TestShouldRecordAccountHealthForwardErrorSkipsNonAccountFaults(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "user parameter 400", err: errors.New("upstream error: 400 Unsupported parameter: max_output_tokens")},
		{name: "invalid input shape", err: errors.New("upstream error: 400 Input must be a list")},
		{name: "model not found", err: errors.New("upstream error: model_not_found")},
		{name: "unsupported model", err: errors.New("upstream error: unsupported model gpt-test")},
		{name: "model mapping", err: errors.New("model mapping does not support requested model")},
		{name: "client canceled", err: context.Canceled},
		{name: "account concurrency", err: errors.New("concurrency limit exceeded for account")},
		{name: "user concurrency", err: errors.New("concurrency limit exceeded for user")},
		{name: "local wait queue", err: errors.New("Too many pending requests, please retry later")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.False(t, shouldRecordAccountHealthForwardError(tt.err))
		})
	}
}

func TestShouldRecordAccountHealthForwardErrorKeepsRealUpstreamFaults(t *testing.T) {
	for _, err := range []error{
		errors.New("upstream error: 503 service unavailable"),
		errors.New("connection refused"),
		errors.New("upstream response failed: timeout"),
	} {
		require.True(t, shouldRecordAccountHealthForwardError(err), err.Error())
	}
}

func TestShouldRecordAccountHealthFailureSkipsNonAccountFaults(t *testing.T) {
	tests := []struct {
		name string
		err  *UpstreamFailoverError
		want bool
	}{
		{name: "parameter 400", err: &UpstreamFailoverError{StatusCode: 400, ResponseBody: []byte(`{"error":{"message":"Unsupported parameter: max_output_tokens"}}`)}, want: false},
		{name: "model 404", err: &UpstreamFailoverError{StatusCode: 404, ResponseBody: []byte(`{"error":{"code":"model_not_found"}}`)}, want: false},
		{name: "unsupported model 503 body", err: &UpstreamFailoverError{StatusCode: 503, ResponseBody: []byte(`{"error":{"message":"unsupported model"}}`)}, want: false},
		{name: "upstream 503", err: &UpstreamFailoverError{StatusCode: 503, ResponseBody: []byte(`{"error":{"message":"service unavailable"}}`)}, want: true},
		{name: "rate limit", err: &UpstreamFailoverError{StatusCode: 429}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldRecordAccountHealthFailure(tt.err))
		})
	}
}
