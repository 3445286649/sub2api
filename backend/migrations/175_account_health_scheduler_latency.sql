-- Separate scheduler latency from full request duration so long generations do not
-- distort account selection. Existing latency_ewma_ms remains an observation-only
-- duration metric for backward-compatible admin displays and diagnostics.

ALTER TABLE account_health_states
    ADD COLUMN IF NOT EXISTS scheduler_latency_ewma_ms INTEGER,
    ADD COLUMN IF NOT EXISTS scheduler_latency_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS consecutive_high_latency INTEGER NOT NULL DEFAULT 0;
