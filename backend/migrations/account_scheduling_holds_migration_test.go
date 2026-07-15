package migrations_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountSchedulingHoldsMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("177_account_scheduling_holds.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	require.Contains(t, sql, "external_scheduling_hold_until")
	require.Contains(t, sql, "create table if not exists account_scheduling_holds")
	require.Contains(t, sql, "unique (account_id, owner)")
	require.Contains(t, sql, "create table if not exists account_scheduling_hold_events")
	require.Contains(t, sql, "unique (owner, decision_id)")
	require.Contains(t, sql, "on delete cascade")
	require.Contains(t, sql, "status in ('active', 'released', 'expired')")
}
