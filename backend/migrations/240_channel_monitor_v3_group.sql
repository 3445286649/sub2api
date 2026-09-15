ALTER TABLE channel_monitors ADD COLUMN IF NOT EXISTS v3_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_channel_monitors_v3_group ON channel_monitors(v3_group_id) WHERE v3_group_id IS NOT NULL;
