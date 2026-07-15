ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS external_scheduling_hold_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_accounts_external_scheduling_hold_until
    ON accounts (external_scheduling_hold_until)
    WHERE external_scheduling_hold_until IS NOT NULL;

CREATE TABLE IF NOT EXISTS account_scheduling_holds (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    owner VARCHAR(32) NOT NULL,
    decision_id VARCHAR(64) NOT NULL,
    reason_code VARCHAR(64) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    lease_until TIMESTAMPTZ NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    expected_account_updated_at TIMESTAMPTZ,
    first_held_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_account_scheduling_holds_account_owner UNIQUE (account_id, owner),
    CONSTRAINT chk_account_scheduling_holds_status CHECK (status IN ('active', 'released', 'expired')),
    CONSTRAINT chk_account_scheduling_holds_lease CHECK (lease_until > first_held_at)
);

CREATE INDEX IF NOT EXISTS idx_account_scheduling_holds_active_lease
    ON account_scheduling_holds (lease_until, account_id)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS account_scheduling_hold_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    owner VARCHAR(32) NOT NULL,
    decision_id VARCHAR(64) NOT NULL,
    command VARCHAR(16) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    reason_code VARCHAR(64) NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    result_status VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_account_scheduling_hold_events_owner_decision UNIQUE (owner, decision_id),
    CONSTRAINT chk_account_scheduling_hold_events_command CHECK (command IN ('put', 'release', 'expire')),
    CONSTRAINT chk_account_scheduling_hold_events_result CHECK (result_status IN ('active', 'released', 'expired', 'noop'))
);

CREATE INDEX IF NOT EXISTS idx_account_scheduling_hold_events_account_time
    ON account_scheduling_hold_events (account_id, created_at DESC);
