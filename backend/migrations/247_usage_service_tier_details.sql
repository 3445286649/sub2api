-- Request-time tier receipt. NULL preserves the unknown status of historical rows.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS service_tier_details JSONB;
COMMENT ON COLUMN usage_logs.service_tier_details IS 'Frozen group, requested, observed and customer billing service tiers';
