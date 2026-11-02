ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS openai_model_tier_multipliers JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.openai_model_tier_multipliers IS 'Requested public model to Fast/Ultrafast Standard price multiplier within this group';
