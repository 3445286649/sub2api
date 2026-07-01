-- Account-level health probe settings.
-- Defaults keep existing behavior: background probes are enabled for unhealthy accounts,
-- using the built-in exponential backoff when no fixed interval is configured.

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS health_probe_enabled BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS health_probe_interval_minutes INTEGER;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'accounts_health_probe_interval_minutes_non_negative'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_health_probe_interval_minutes_non_negative
                CHECK (health_probe_interval_minutes IS NULL OR health_probe_interval_minutes >= 0);
    END IF;
END $$;

COMMENT ON COLUMN accounts.health_probe_enabled IS 'Whether background account health probes are enabled for this account.';
COMMENT ON COLUMN accounts.health_probe_interval_minutes IS 'Optional fixed background health probe interval in minutes for unhealthy accounts; NULL uses default backoff.';
