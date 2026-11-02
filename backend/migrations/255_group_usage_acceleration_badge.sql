ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS usage_acceleration_badge VARCHAR(32) NOT NULL DEFAULT ''
    CHECK (usage_acceleration_badge IN ('', 'fast', 'ultrafast', 'free_fast', 'billed_fast', 'billed_ultrafast'));

COMMENT ON COLUMN groups.usage_acceleration_badge IS
    'Usage badge override; without an active forced tier requests and billing use Standard';
