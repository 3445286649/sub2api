SET LOCAL lock_timeout = '5s';

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS display_order INTEGER NOT NULL DEFAULT 0
    CHECK (display_order BETWEEN 0 AND 999999);
