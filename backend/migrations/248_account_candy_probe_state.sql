-- Keep candy-probe scheduling independent from health-probe scheduling.
CREATE TABLE IF NOT EXISTS account_candy_probe_states (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    next_probe_at TIMESTAMPTZ,
    lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_candy_probe_states_due
    ON account_candy_probe_states(next_probe_at, lease_until, account_id);

COMMENT ON TABLE account_candy_probe_states IS 'Independent due time and claim lease for scheduled candy account probes.';
