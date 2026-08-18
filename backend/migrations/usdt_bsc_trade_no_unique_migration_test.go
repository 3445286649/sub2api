package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUSDTBSCTradeNoUniqueMigrationIsOnlineAndScoped(t *testing.T) {
	content, err := FS.ReadFile("225_payment_orders_usdt_bsc_trade_no_unique_notx.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS")
	require.Contains(t, sql, "ON payment_orders (payment_trade_no)")
	require.Contains(t, sql, "payment_type = 'usdt_bsc'")
	require.Contains(t, sql, "payment_trade_no <> ''")
}
