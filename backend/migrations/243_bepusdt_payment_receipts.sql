ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS client_request_id VARCHAR(36);
CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_user_request_unique
    ON payment_orders(user_id, client_request_id);

-- No FK cascade: a refunded or removed order must not release its chain receipt.
CREATE TABLE IF NOT EXISTS crypto_payment_receipts (
    id BIGSERIAL PRIMARY KEY,
    chain_id VARCHAR(32) NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    order_id BIGINT NOT NULL,
    token_contract VARCHAR(128) NOT NULL DEFAULT '',
    event_index BIGINT,
    amount_atomic VARCHAR(78) NOT NULL DEFAULT '',
    source VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chain_id, tx_hash)
);
CREATE INDEX IF NOT EXISTS crypto_payment_receipts_order_id_idx ON crypto_payment_receipts(order_id);

CREATE TABLE IF NOT EXISTS payment_callback_nonces (
    id BIGSERIAL PRIMARY KEY,
    provider_instance_id VARCHAR(64) NOT NULL,
    key_id VARCHAR(64) NOT NULL,
    nonce VARCHAR(128) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE(provider_instance_id, key_id, nonce)
);
CREATE INDEX IF NOT EXISTS payment_callback_nonces_expiry_idx ON payment_callback_nonces(expires_at);
