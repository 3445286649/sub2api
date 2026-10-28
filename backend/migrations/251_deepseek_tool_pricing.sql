ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS deepseek_tool_pricing JSONB;
