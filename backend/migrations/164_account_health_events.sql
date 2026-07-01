-- Account health event stream for auditability and troubleshooting.

CREATE TABLE IF NOT EXISTS account_health_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    source VARCHAR(40) NOT NULL,
    event_type VARCHAR(40) NOT NULL,
    score_before INTEGER NOT NULL DEFAULT 0,
    score_after INTEGER NOT NULL DEFAULT 0,
    status_before VARCHAR(32) NOT NULL DEFAULT '',
    status_after VARCHAR(32) NOT NULL DEFAULT '',
    delta INTEGER NOT NULL DEFAULT 0,
    error_category VARCHAR(40),
    error_message TEXT,
    latency_ms BIGINT,
    affected_group_ids BIGINT[],
    actor_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_health_events_account_created
    ON account_health_events(account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_health_events_created
    ON account_health_events(created_at);

CREATE INDEX IF NOT EXISTS idx_account_health_events_type
    ON account_health_events(event_type);
