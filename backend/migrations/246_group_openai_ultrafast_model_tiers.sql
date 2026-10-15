ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS force_openai_ultrafast BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS openai_model_tiers JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.force_openai_ultrafast IS 'Allow model-specific forced OpenAI Ultrafast in this group';
COMMENT ON COLUMN groups.openai_model_tiers IS 'Requested public model to forced service tier within this group';
