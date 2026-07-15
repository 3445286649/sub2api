package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountExternalSchedulingHoldProjection(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)

	t.Run("active hold blocks normal selection", func(t *testing.T) {
		account := &Account{Status: StatusActive, Schedulable: true, ExternalSchedulingHoldUntil: &future}
		require.False(t, account.IsSchedulable())
	})

	t.Run("expired hold recovers without mutating manual state", func(t *testing.T) {
		account := &Account{Status: StatusActive, Schedulable: true, ExternalSchedulingHoldUntil: &past}
		require.True(t, account.IsSchedulable())
	})

	t.Run("active parent hold blocks spark credential reuse", func(t *testing.T) {
		account := &Account{Status: StatusActive, Schedulable: true, ExternalSchedulingHoldUntil: &future}
		require.False(t, account.IsCredentialUsableForShadow())
	})
}
