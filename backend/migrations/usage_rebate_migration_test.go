package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageRebateMigrationKeepsFinancialIdempotencyAndAuditFields(t *testing.T) {
	content, err := FS.ReadFile("182_usage_rebate.sql")
	require.NoError(t, err)
	sql := string(content)

	for _, fragment := range []string{
		"business_date DATE NOT NULL UNIQUE",
		"business_key VARCHAR(128) NOT NULL UNIQUE",
		"UNIQUE (period_id, user_id)",
		"UNIQUE (period_id, rank)",
		"status IN ('pending', 'credited', 'failed', 'unknown')",
		"balance_before NUMERIC(20, 8)",
		"balance_after NUMERIC(20, 8)",
	} {
		require.True(t, strings.Contains(sql, fragment), fragment)
	}
	require.NotContains(t, sql, "total_recharged")
}
