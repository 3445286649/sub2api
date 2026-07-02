-- Cached upstream wallet/quota balance by normalized account base URL.
-- The cache is operational display data only; it must not affect scheduling or account health.

CREATE TABLE IF NOT EXISTS account_upstream_balance_snapshots (
    base_url TEXT PRIMARY KEY,
    representative_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'unsupported',
    balance NUMERIC(20, 8),
    remaining NUMERIC(20, 8),
    unit VARCHAR(16) NOT NULL DEFAULT '',
    source_endpoint TEXT NOT NULL DEFAULT '',
    http_status INTEGER,
    error_message TEXT,
    checked_at TIMESTAMPTZ,
    next_check_at TIMESTAMPTZ,
    claim_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_upstream_balance_due
    ON account_upstream_balance_snapshots(next_check_at)
    WHERE next_check_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_account_upstream_balance_claim
    ON account_upstream_balance_snapshots(claim_until)
    WHERE claim_until IS NOT NULL;
