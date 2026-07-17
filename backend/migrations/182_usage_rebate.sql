CREATE TABLE IF NOT EXISTS usage_rebate_periods (
    id BIGSERIAL PRIMARY KEY,
    business_date DATE NOT NULL UNIQUE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    settle_after TIMESTAMPTZ NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    rule_version VARCHAR(32) NOT NULL,
    rates JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    total_spend NUMERIC(20, 8) NOT NULL DEFAULT 0,
    total_reward NUMERIC(20, 8) NOT NULL DEFAULT 0,
    attempt_count INT NOT NULL DEFAULT 0,
    lock_token VARCHAR(64),
    locked_until TIMESTAMPTZ,
    sealed_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,
    error_message VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT usage_rebate_periods_window_check CHECK (window_end > window_start),
    CONSTRAINT usage_rebate_periods_status_check CHECK (status IN ('open', 'settling', 'settled', 'failed', 'unknown'))
);

CREATE INDEX IF NOT EXISTS idx_usage_rebate_periods_due
    ON usage_rebate_periods (settle_after, id)
    WHERE status IN ('open', 'settling', 'failed');

CREATE TABLE IF NOT EXISTS usage_rebate_rewards (
    id BIGSERIAL PRIMARY KEY,
    period_id BIGINT NOT NULL REFERENCES usage_rebate_periods(id) ON DELETE RESTRICT,
    business_date DATE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    rank INT NOT NULL,
    spend_amount NUMERIC(20, 8) NOT NULL,
    rebate_percent NUMERIC(8, 4) NOT NULL,
    reward_amount NUMERIC(20, 8) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    business_key VARCHAR(128) NOT NULL UNIQUE,
    balance_before NUMERIC(20, 8),
    balance_after NUMERIC(20, 8),
    error_message VARCHAR(500) NOT NULL DEFAULT '',
    credited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT usage_rebate_rewards_period_user_unique UNIQUE (period_id, user_id),
    CONSTRAINT usage_rebate_rewards_period_rank_unique UNIQUE (period_id, rank),
    CONSTRAINT usage_rebate_rewards_rank_check CHECK (rank BETWEEN 1 AND 20),
    CONSTRAINT usage_rebate_rewards_amount_check CHECK (spend_amount > 0 AND reward_amount > 0),
    CONSTRAINT usage_rebate_rewards_status_check CHECK (status IN ('pending', 'credited', 'failed', 'unknown'))
);

CREATE INDEX IF NOT EXISTS idx_usage_rebate_rewards_user_created
    ON usage_rebate_rewards (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_usage_rebate_rewards_period_status
    ON usage_rebate_rewards (period_id, status, id);
