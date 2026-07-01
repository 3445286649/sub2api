-- Add optional low-frequency probe settings for healthy accounts.
-- Defaults keep current behavior: healthy accounts are not proactively probed.

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS healthy_probe_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS healthy_probe_interval_hours INTEGER;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'accounts_healthy_probe_interval_hours_positive'
  ) THEN
    ALTER TABLE accounts
      ADD CONSTRAINT accounts_healthy_probe_interval_hours_positive
      CHECK (healthy_probe_interval_hours IS NULL OR healthy_probe_interval_hours > 0);
  END IF;
END $$;
