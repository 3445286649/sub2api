CREATE TABLE IF NOT EXISTS redeem_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    expires_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_redeem_campaigns_status
    ON redeem_campaigns(status);
CREATE INDEX IF NOT EXISTS idx_redeem_campaigns_expires_at
    ON redeem_campaigns(expires_at);

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS campaign_id BIGINT NULL REFERENCES redeem_campaigns(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_campaign_id
    ON redeem_codes(campaign_id);

CREATE TABLE IF NOT EXISTS redeem_campaign_redemptions (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES redeem_campaigns(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    redeem_code_id BIGINT NULL REFERENCES redeem_codes(id) ON DELETE SET NULL,
    redeemed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT redeem_campaign_redemptions_campaign_user_key UNIQUE (campaign_id, user_id),
    CONSTRAINT redeem_campaign_redemptions_redeem_code_key UNIQUE (redeem_code_id)
);

CREATE INDEX IF NOT EXISTS idx_redeem_campaign_redemptions_user_id
    ON redeem_campaign_redemptions(user_id);
