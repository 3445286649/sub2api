-- Enforce one chain transaction hash per USDT-BSC payment order.
-- The migration runner performs a duplicate precheck before this online index build.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS payment_orders_usdt_bsc_trade_no_unique
    ON payment_orders (payment_trade_no)
    WHERE payment_type = 'usdt_bsc' AND payment_trade_no <> '';
