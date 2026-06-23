-- Add subscription plan product typing and payment-order fulfillment snapshots.
ALTER TABLE subscription_plans
  ADD COLUMN IF NOT EXISTS plan_type varchar(30) NOT NULL DEFAULT 'subscription',
  ADD COLUMN IF NOT EXISTS quota_reset_scope varchar(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS quota_reset_value decimal(20,2) NOT NULL DEFAULT 0;

ALTER TABLE payment_orders
  ADD COLUMN IF NOT EXISTS subscription_plan_type varchar(30) NOT NULL DEFAULT 'subscription',
  ADD COLUMN IF NOT EXISTS quota_reset_scope varchar(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS quota_reset_value decimal(20,2) NOT NULL DEFAULT 0;
