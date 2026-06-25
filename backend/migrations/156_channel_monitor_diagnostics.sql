ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS failure_category VARCHAR(40) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS http_status INT,
    ADD COLUMN IF NOT EXISTS request_path VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_channel_monitor_histories_failure_category
    ON channel_monitor_histories (failure_category);
