-- Only signature-verified callbacks create tasks; never backfill historical orders.
CREATE TABLE IF NOT EXISTS payment_fm_reviews (
    order_id BIGINT PRIMARY KEY REFERENCES payment_orders(id) ON DELETE CASCADE,
    provider_instance_id BIGINT NOT NULL REFERENCES payment_provider_instances(id),
    merchant_id VARCHAR(100) NOT NULL,
    trade_no VARCHAR(100) NOT NULL,
    amount DOUBLE PRECISION NOT NULL CHECK (amount > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'done', 'review')),
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL,
    deadline TIMESTAMPTZ NOT NULL,
    lease_token VARCHAR(36),
    leased_until TIMESTAMPTZ,
    last_error VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (merchant_id, trade_no)
);
CREATE INDEX IF NOT EXISTS payment_fm_reviews_ready_idx ON payment_fm_reviews(status, available_at);
CREATE TABLE IF NOT EXISTS payment_fm_query_limits (
    merchant_id VARCHAR(100) PRIMARY KEY,
    next_allowed_at TIMESTAMPTZ NOT NULL
);

-- A platform transaction cannot credit multiple orders for the same merchant,
-- including signed-callback-only instances and refunded historical payments.
CREATE UNIQUE INDEX IF NOT EXISTS payment_fm_paid_trade_unique
    ON payment_orders ((provider_snapshot ->> 'merchant_id'), payment_trade_no)
    WHERE provider_key = 'zhifufm' AND payment_trade_no <> '' AND paid_at IS NOT NULL;
