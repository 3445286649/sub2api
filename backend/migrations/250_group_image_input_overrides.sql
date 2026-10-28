ALTER TABLE groups
ADD COLUMN IF NOT EXISTS image_input_overrides jsonb NOT NULL DEFAULT '{}'::jsonb;
