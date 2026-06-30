DO $$
DECLARE
    duplicate_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO duplicate_count
    FROM (
        SELECT source_order_id
        FROM user_affiliate_ledger
        WHERE action = 'accrue'
          AND source_order_id IS NOT NULL
        GROUP BY source_order_id
        HAVING COUNT(*) > 1
    ) duplicate_orders;

    IF duplicate_count > 0 THEN
        RAISE EXCEPTION
            'user_affiliate_ledger has duplicate accrue rows for % source_order_id value(s); clean duplicates before applying 159_affiliate_order_ledger_idempotency.sql',
            duplicate_count;
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_source_order_accrue_uniq
    ON user_affiliate_ledger(source_order_id)
    WHERE action = 'accrue' AND source_order_id IS NOT NULL;
