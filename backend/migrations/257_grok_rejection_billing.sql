-- Financial receipt IDs survive usage-log retention, account/key deletion and restarts.
CREATE TABLE IF NOT EXISTS grok_rejection_receipts (
    source_id VARCHAR(64) NOT NULL,
    billing_request_id VARCHAR(255) NOT NULL,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    upstream_amount NUMERIC(20,8) NOT NULL CHECK (upstream_amount > 0),
    user_amount NUMERIC(20,8) NOT NULL CHECK (user_amount > 0),
    usage_request_id VARCHAR(255) NOT NULL,
    evidence JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_id, billing_request_id)
);

-- Durable inbox: recovery retries settlement, never replays the paid upstream request.
CREATE TABLE IF NOT EXISTS grok_rejection_inbox (
    event_key VARCHAR(64) PRIMARY KEY,
    payload JSONB NOT NULL,
    state VARCHAR(16) NOT NULL CHECK (state IN ('pending','review','settled','duplicate','conflict')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS grok_rejection_pending ON grok_rejection_inbox(next_attempt_at) WHERE state = 'pending';

CREATE TABLE IF NOT EXISTS grok_rejection_tasks (
    source_id VARCHAR(64) NOT NULL,
    task_id VARCHAR(255) NOT NULL,
	 normal_billing_replaced BOOLEAN NOT NULL DEFAULT FALSE,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_id, task_id)
);
