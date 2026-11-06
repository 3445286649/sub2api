-- Renewal mode captured on a paid subscription order: 'restart' replaces the
-- current term, 'extend' adds days to it. Existing orders keep the old
-- accumulate semantics, hence the 'extend' default.
ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS subscription_renewal_mode VARCHAR(16) NOT NULL DEFAULT 'extend';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'payment_orders_subscription_renewal_mode_check'
          AND conrelid = 'payment_orders'::regclass
    ) THEN
        ALTER TABLE payment_orders
            ADD CONSTRAINT payment_orders_subscription_renewal_mode_check
            CHECK (subscription_renewal_mode IN ('restart', 'extend'));
    END IF;
END $$;
