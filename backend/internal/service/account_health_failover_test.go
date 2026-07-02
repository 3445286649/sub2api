//go:build unit

package service

import (
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
