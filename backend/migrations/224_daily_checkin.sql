CREATE TABLE IF NOT EXISTS daily_checkin_cycles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cycle_number INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed')),
    cycle_days INTEGER NOT NULL CHECK (cycle_days BETWEEN 1 AND 365),
    checkin_count INTEGER NOT NULL DEFAULT 0 CHECK (checkin_count >= 0),
    consecutive_days INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_days >= 0),
    started_on DATE NOT NULL,
    last_checkin_on DATE NULL,
    completed_at TIMESTAMPTZ NULL,
    base_reward NUMERIC(20,8) NOT NULL CHECK (base_reward >= 0),
    milestone_7_reward NUMERIC(20,8) NOT NULL CHECK (milestone_7_reward >= 0),
    milestone_15_reward NUMERIC(20,8) NOT NULL CHECK (milestone_15_reward >= 0),
    milestone_30_reward NUMERIC(20,8) NOT NULL CHECK (milestone_30_reward >= 0),
    rule_version INTEGER NOT NULL DEFAULT 1 CHECK (rule_version > 0),
    total_reward NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (total_reward >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, cycle_number)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_daily_checkin_active_cycle
    ON daily_checkin_cycles(user_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_daily_checkin_cycles_user_created
    ON daily_checkin_cycles(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS daily_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cycle_id BIGINT NOT NULL REFERENCES daily_checkin_cycles(id) ON DELETE RESTRICT,
    business_date DATE NOT NULL,
    cycle_day INTEGER NOT NULL CHECK (cycle_day > 0),
    base_reward NUMERIC(20,8) NOT NULL CHECK (base_reward >= 0),
    milestone_reward NUMERIC(20,8) NOT NULL CHECK (milestone_reward >= 0),
    total_reward NUMERIC(20,8) NOT NULL CHECK (total_reward >= 0),
    balance_before NUMERIC(20,8) NOT NULL,
    balance_after NUMERIC(20,8) NOT NULL,
    rule_version INTEGER NOT NULL CHECK (rule_version > 0),
    business_key VARCHAR(120) NOT NULL UNIQUE,
    client_ip INET NULL,
    user_agent_hash VARCHAR(128) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, business_date),
    UNIQUE (cycle_id, cycle_day)
);

CREATE INDEX IF NOT EXISTS idx_daily_checkins_user_created
    ON daily_checkins(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_daily_checkins_business_date
    ON daily_checkins(business_date DESC);

CREATE TABLE IF NOT EXISTS daily_checkin_daily_totals (
    business_date DATE PRIMARY KEY,
    claim_count BIGINT NOT NULL DEFAULT 0 CHECK (claim_count >= 0),
    total_reward NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (total_reward >= 0),
    budget_limit NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (budget_limit >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings(key, value, updated_at) VALUES
    ('daily_checkin_enabled', 'false', NOW()),
    ('daily_checkin_base_reward', '0.13', NOW()),
    ('daily_checkin_cycle_days', '30', NOW()),
    ('daily_checkin_milestone_7', '2', NOW()),
    ('daily_checkin_milestone_15', '5', NOW()),
    ('daily_checkin_milestone_30', '8', NOW()),
    ('daily_checkin_min_account_age_hours', '0', NOW()),
    ('daily_checkin_require_verified', 'false', NOW()),
    ('daily_checkin_daily_budget', '0', NOW()),
    ('daily_checkin_rule_version', '1', NOW())
ON CONFLICT (key) DO NOTHING;

COMMENT ON TABLE daily_checkin_cycles IS 'Per-user 30-day check-in cycles with immutable reward rule snapshots';
COMMENT ON TABLE daily_checkins IS 'Idempotent daily check-in grants and balance snapshots';
