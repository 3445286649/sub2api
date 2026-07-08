-- Add minute-level interval for low-frequency probes while an account is healthy.
-- The legacy healthy_probe_interval_hours column is kept as a compatibility fallback.

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS healthy_probe_interval_minutes INTEGER;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'accounts_healthy_probe_interval_minutes_positive'
  ) THEN
    ALTER TABLE accounts
      ADD CONSTRAINT accounts_healthy_probe_interval_minutes_positive
      CHECK (healthy_probe_interval_minutes IS NULL OR healthy_probe_interval_minutes > 0);
  END IF;
END $$;
