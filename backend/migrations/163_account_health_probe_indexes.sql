-- Indexes for account health background probe candidate queries.
-- These are additive and safe to re-run.

CREATE INDEX IF NOT EXISTS idx_account_health_states_due_probe_status_next
    ON account_health_states(status, next_probe_at)
    WHERE next_probe_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_accounts_healthy_probe_candidates
    ON accounts(updated_at, id)
    WHERE deleted_at IS NULL
      AND status = 'active'
      AND schedulable IS TRUE
      AND health_probe_enabled IS TRUE
      AND healthy_probe_enabled IS TRUE;
