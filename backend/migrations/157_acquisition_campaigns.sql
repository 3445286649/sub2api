CREATE TABLE IF NOT EXISTS acquisition_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    leaderboard_enabled BOOLEAN NOT NULL DEFAULT true,
    lottery_enabled BOOLEAN NOT NULL DEFAULT true,
    leaderboard_pool_usd DECIMAL(20,8) NOT NULL DEFAULT 0,
    leaderboard_shares JSONB NOT NULL DEFAULT '[40,25,15,12,8]'::jsonb,
    lottery_prize_configs JSONB NOT NULL DEFAULT '[]'::jsonb,
    lottery_seed VARCHAR(128) NOT NULL DEFAULT '',
    settled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_acquisition_campaigns_status_ends
    ON acquisition_campaigns(status, ends_at);
CREATE INDEX IF NOT EXISTS idx_acquisition_campaigns_active_window
    ON acquisition_campaigns(starts_at, ends_at)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS acquisition_participations (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES acquisition_campaigns(id) ON DELETE CASCADE,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE CASCADE,
    completed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_acquisition_participations_campaign_invitee
    ON acquisition_participations(campaign_id, invitee_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_acquisition_participations_campaign_order
    ON acquisition_participations(campaign_id, source_order_id);
CREATE INDEX IF NOT EXISTS idx_acquisition_participations_campaign_inviter
    ON acquisition_participations(campaign_id, inviter_id, completed_at);

CREATE TABLE IF NOT EXISTS acquisition_lottery_tickets (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES acquisition_campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE CASCADE,
    ticket_role VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_acquisition_tickets_unique_role
    ON acquisition_lottery_tickets(campaign_id, source_order_id, ticket_role);
CREATE INDEX IF NOT EXISTS idx_acquisition_tickets_campaign_user
    ON acquisition_lottery_tickets(campaign_id, user_id);

CREATE TABLE IF NOT EXISTS acquisition_rewards (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES acquisition_campaigns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_type VARCHAR(30) NOT NULL,
    reward_key VARCHAR(160) NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    rank INTEGER NOT NULL DEFAULT 0,
    prize_name VARCHAR(120) NOT NULL DEFAULT '',
    ticket_id BIGINT NULL REFERENCES acquisition_lottery_tickets(id) ON DELETE SET NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    paid_at TIMESTAMPTZ NULL,
    balance_after DECIMAL(20,8) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_acquisition_rewards_unique_key
    ON acquisition_rewards(campaign_id, reward_type, reward_key);
CREATE INDEX IF NOT EXISTS idx_acquisition_rewards_campaign_status
    ON acquisition_rewards(campaign_id, status);
CREATE INDEX IF NOT EXISTS idx_acquisition_rewards_user
    ON acquisition_rewards(user_id, created_at DESC);

INSERT INTO settings (key, value, updated_at)
VALUES
    ('acquisition_enabled', 'false', NOW()),
    ('acquisition_leaderboard_enabled', 'true', NOW()),
    ('acquisition_lottery_enabled', 'true', NOW())
ON CONFLICT (key) DO NOTHING;

COMMENT ON TABLE acquisition_campaigns IS '拉新活动周期配置';
COMMENT ON TABLE acquisition_participations IS '拉新活动有效拉新完成记录';
COMMENT ON TABLE acquisition_lottery_tickets IS '拉新活动抽奖券';
COMMENT ON TABLE acquisition_rewards IS '拉新活动排行榜/抽奖奖励与发放审计';
