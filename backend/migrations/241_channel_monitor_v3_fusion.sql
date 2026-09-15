-- Opt-in only. The next due time prevents staggered replicas from duplicating probes.
ALTER TABLE channel_monitors ADD COLUMN IF NOT EXISTS v3_fusion_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE channel_monitors ADD COLUMN IF NOT EXISTS probe_next_check_at TIMESTAMPTZ;
-- Historical quota-only snapshots must never be interpreted as successful probes.
ALTER TABLE channel_monitor_histories ADD COLUMN IF NOT EXISTS is_probe BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE channel_monitor_histories ADD COLUMN IF NOT EXISTS probe_target_hash VARCHAR(64) NOT NULL DEFAULT '';
