-- Optional account-level model used by health probes.
-- NULL keeps existing behavior: use the platform default test model.

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS health_probe_model TEXT;

COMMENT ON COLUMN accounts.health_probe_model IS 'Optional account-level model used by health probes; NULL uses platform default test model.';
