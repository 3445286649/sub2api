SET LOCAL lock_timeout = '5s';

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS display_category VARCHAR(50) NOT NULL DEFAULT '';
