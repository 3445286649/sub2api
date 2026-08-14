CREATE TABLE IF NOT EXISTS user_points_accounts (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    available_points BIGINT NOT NULL DEFAULT 0 CHECK (available_points >= 0),
    frozen_points BIGINT NOT NULL DEFAULT 0 CHECK (frozen_points >= 0),
    debt_points BIGINT NOT NULL DEFAULT 0 CHECK (debt_points >= 0),
    lifetime_earned BIGINT NOT NULL DEFAULT 0 CHECK (lifetime_earned >= 0),
    lifetime_spent BIGINT NOT NULL DEFAULT 0 CHECK (lifetime_spent >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS affiliate_point_awards (
    id BIGSERIAL PRIMARY KEY,
    inviter_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_order_id BIGINT NULL REFERENCES payment_orders(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'available', 'revoked')),
    points BIGINT NOT NULL CHECK (points > 0),
    threshold_amount DECIMAL(20,8) NOT NULL CHECK (threshold_amount > 0),
    qualifying_amount DECIMAL(20,8) NOT NULL CHECK (qualifying_amount >= 0),
    qualification_window_days INTEGER NOT NULL CHECK (qualification_window_days > 0),
    freeze_hours INTEGER NOT NULL CHECK (freeze_hours >= 0),
    award_version INTEGER NOT NULL DEFAULT 1 CHECK (award_version > 0),
    release_at TIMESTAMPTZ NOT NULL,
    released_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (invitee_user_id)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_point_awards_inviter_status_release
    ON affiliate_point_awards(inviter_user_id, status, release_at);

CREATE TABLE IF NOT EXISTS points_shop_products (
    id BIGSERIAL PRIMARY KEY,
    product_type VARCHAR(20) NOT NULL DEFAULT 'balance' CHECK (product_type = 'balance'),
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    points_price BIGINT NOT NULL CHECK (points_price > 0),
    original_points_price BIGINT NULL CHECK (original_points_price IS NULL OR original_points_price >= points_price),
    balance_amount DECIMAL(20,8) NOT NULL CHECK (balance_amount > 0),
    stock_total BIGINT NULL CHECK (stock_total IS NULL OR stock_total >= 0),
    stock_redeemed BIGINT NOT NULL DEFAULT 0 CHECK (stock_redeemed >= 0),
    per_user_limit INTEGER NULL CHECK (per_user_limit IS NULL OR per_user_limit > 0),
    features TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    for_sale BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_points_shop_products_sale_sort
    ON points_shop_products(for_sale, sort_order, id);

CREATE TABLE IF NOT EXISTS points_shop_orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(40) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    product_id BIGINT NULL REFERENCES points_shop_products(id) ON DELETE SET NULL,
    idempotency_key VARCHAR(64) NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    product_type VARCHAR(20) NOT NULL,
    points_price BIGINT NOT NULL CHECK (points_price > 0),
    balance_amount DECIMAL(20,8) NOT NULL CHECK (balance_amount > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'reversed')),
    balance_after DECIMAL(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_points_shop_orders_user_created
    ON points_shop_orders(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS user_points_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(24) NOT NULL CHECK (action IN ('earn_pending', 'earn_release', 'earn_revoke', 'redeem')),
    delta_available BIGINT NOT NULL DEFAULT 0,
    delta_frozen BIGINT NOT NULL DEFAULT 0,
    delta_debt BIGINT NOT NULL DEFAULT 0,
    available_after BIGINT NOT NULL CHECK (available_after >= 0),
    frozen_after BIGINT NOT NULL CHECK (frozen_after >= 0),
    debt_after BIGINT NOT NULL CHECK (debt_after >= 0),
    source_type VARCHAR(24) NOT NULL,
    source_id BIGINT NULL,
    business_key VARCHAR(100) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_points_ledger_user_created
    ON user_points_ledger(user_id, created_at DESC);

INSERT INTO settings(key, value, updated_at) VALUES
    ('points_shop_enabled', 'true', NOW()),
    ('points_invite_threshold_amount', '50', NOW()),
    ('points_invite_reward_points', '1', NOW()),
    ('points_invite_window_days', '30', NOW()),
    ('points_invite_freeze_hours', '168', NOW())
ON CONFLICT (key) DO NOTHING;

COMMENT ON TABLE user_points_ledger IS 'Immutable points ledger for invite rewards and shop redemption';
COMMENT ON TABLE affiliate_point_awards IS 'One active invite qualification award per invitee with rule snapshots';
COMMENT ON TABLE points_shop_orders IS 'Immutable points-shop fulfillment snapshot';
