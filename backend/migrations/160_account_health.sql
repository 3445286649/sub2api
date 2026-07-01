-- Account-level upstream health state.
-- Health is keyed by account_id so the same upstream URL with different keys is isolated independently.

CREATE TABLE IF NOT EXISTS account_health_states (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    score INTEGER NOT NULL DEFAULT 80 CHECK (score >= 0 AND score <= 100),
    consecutive_successes INTEGER NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'healthy',
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    last_checked_at TIMESTAMPTZ,
    last_error_category VARCHAR(40),
    last_error_message TEXT,
    latency_ewma_ms INTEGER,
    backoff_level INTEGER NOT NULL DEFAULT 0,
    next_probe_at TIMESTAMPTZ,
    isolated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_health_states_status
    ON account_health_states(status);

CREATE INDEX IF NOT EXISTS idx_account_health_states_next_probe
    ON account_health_states(next_probe_at)
    WHERE next_probe_at IS NOT NULL;
