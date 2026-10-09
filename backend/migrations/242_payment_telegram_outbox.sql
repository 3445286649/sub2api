CREATE TABLE IF NOT EXISTS payment_telegram_outbox (
    id BIGSERIAL PRIMARY KEY,
    event_key VARCHAR(200) NOT NULL UNIQUE,
    order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    chat_id_snapshot VARCHAR(32) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sending', 'sent', 'permanent_error')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    leased_until TIMESTAMPTZ,
    lease_token VARCHAR(36),
    last_error VARCHAR(200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS payment_telegram_outbox_ready_idx
    ON payment_telegram_outbox (available_at, id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS payment_telegram_outbox_lease_idx
    ON payment_telegram_outbox (leased_until, id)
    WHERE status = 'sending';
