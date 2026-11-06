-- Lifetime public-signup quota per source IP (IPv4 address or IPv6 /64).
-- No user FK: deleting an account must not refund the quota. Rows are updated
-- in the SAME transaction as user creation. Existing accounts have no reliable
-- registration IP and are not backfilled.
-- created_at is diagnostic only; quotas must never expire.
CREATE TABLE IF NOT EXISTS registration_ip_quotas (
    ip_hash VARCHAR(64) PRIMARY KEY CHECK (length(ip_hash) = 64),
    account_count SMALLINT NOT NULL CHECK (account_count BETWEEN 1 AND 2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
