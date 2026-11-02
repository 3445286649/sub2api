CREATE TABLE IF NOT EXISTS user_group_account_restrictions (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, group_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_user_group_account_restrictions_account
    ON user_group_account_restrictions (account_id);
CREATE INDEX IF NOT EXISTS idx_user_group_account_restrictions_group
    ON user_group_account_restrictions (group_id);
