ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS points_rule_threshold_amount DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS points_rule_reward_points BIGINT,
    ADD COLUMN IF NOT EXISTS points_rule_window_days INTEGER,
    ADD COLUMN IF NOT EXISTS points_rule_freeze_hours INTEGER,
    ADD COLUMN IF NOT EXISTS points_rule_version INTEGER;

-- Existing awards are the authoritative historical snapshot.
UPDATE user_affiliates ua
SET points_rule_threshold_amount = award.threshold_amount,
    points_rule_reward_points = award.points,
    points_rule_window_days = award.qualification_window_days,
    points_rule_freeze_hours = award.freeze_hours,
    points_rule_version = 1
FROM affiliate_point_awards award
WHERE award.invitee_user_id = ua.user_id
  AND ua.inviter_id IS NOT NULL
  AND ua.points_rule_threshold_amount IS NULL;

-- Relationships created before rule snapshots shipped used the original
-- points-shop rule. Do not reinterpret them using settings changed later.
UPDATE user_affiliates
SET points_rule_threshold_amount = 50,
    points_rule_reward_points = 1,
    points_rule_window_days = 30,
    points_rule_freeze_hours = 168,
    points_rule_version = 1
WHERE inviter_id IS NOT NULL
  AND points_rule_threshold_amount IS NULL;

CREATE OR REPLACE FUNCTION public.capture_affiliate_points_rule_snapshot()
RETURNS TRIGGER
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $snapshot$
BEGIN
    IF NEW.inviter_id IS NOT NULL THEN
        NEW.points_rule_threshold_amount := COALESCE(
            NEW.points_rule_threshold_amount,
            (
                SELECT CASE
                    WHEN value ~ '^[0-9]+([.][0-9]+)?$' THEN NULLIF(value::numeric, 0)
                    ELSE NULL
                END
                FROM public.settings
                WHERE key = 'points_invite_threshold_amount'
                LIMIT 1
            ),
            50
        );
        NEW.points_rule_reward_points := COALESCE(
            NEW.points_rule_reward_points,
            (
                SELECT CASE
                    WHEN value ~ '^[0-9]+$' THEN NULLIF(value::bigint, 0)
                    ELSE NULL
                END
                FROM public.settings
                WHERE key = 'points_invite_reward_points'
                LIMIT 1
            ),
            1
        );
        NEW.points_rule_window_days := COALESCE(
            NEW.points_rule_window_days,
            (
                SELECT CASE
                    WHEN value ~ '^[0-9]+$' THEN NULLIF(value::integer, 0)
                    ELSE NULL
                END
                FROM public.settings
                WHERE key = 'points_invite_window_days'
                LIMIT 1
            ),
            30
        );
        NEW.points_rule_freeze_hours := COALESCE(
            NEW.points_rule_freeze_hours,
            (
                SELECT CASE
                    WHEN value ~ '^[0-9]+$' THEN value::integer
                    ELSE NULL
                END
                FROM public.settings
                WHERE key = 'points_invite_freeze_hours'
                LIMIT 1
            ),
            168
        );
        NEW.points_rule_version := COALESCE(NEW.points_rule_version, 1);
    END IF;
    RETURN NEW;
END
$snapshot$;

DROP TRIGGER IF EXISTS trg_user_affiliates_points_rule_snapshot ON user_affiliates;
CREATE TRIGGER trg_user_affiliates_points_rule_snapshot
BEFORE INSERT OR UPDATE OF inviter_id ON user_affiliates
FOR EACH ROW
EXECUTE FUNCTION public.capture_affiliate_points_rule_snapshot();

DO $constraints$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'user_affiliates_points_rule_complete'
          AND conrelid = 'public.user_affiliates'::regclass
    ) THEN
        ALTER TABLE user_affiliates
            ADD CONSTRAINT user_affiliates_points_rule_complete CHECK (
                inviter_id IS NULL OR (
                    points_rule_threshold_amount IS NOT NULL
                    AND points_rule_reward_points IS NOT NULL
                    AND points_rule_window_days IS NOT NULL
                    AND points_rule_freeze_hours IS NOT NULL
                    AND points_rule_version IS NOT NULL
                )
            ) NOT VALID;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'user_affiliates_points_rule_values'
          AND conrelid = 'public.user_affiliates'::regclass
    ) THEN
        ALTER TABLE user_affiliates
            ADD CONSTRAINT user_affiliates_points_rule_values CHECK (
                (points_rule_threshold_amount IS NULL OR points_rule_threshold_amount > 0)
                AND (points_rule_reward_points IS NULL OR points_rule_reward_points > 0)
                AND (points_rule_window_days IS NULL OR points_rule_window_days > 0)
                AND (points_rule_freeze_hours IS NULL OR points_rule_freeze_hours >= 0)
                AND (points_rule_version IS NULL OR points_rule_version > 0)
            ) NOT VALID;
    END IF;
END
$constraints$;

ALTER TABLE user_affiliates VALIDATE CONSTRAINT user_affiliates_points_rule_complete;
ALTER TABLE user_affiliates VALIDATE CONSTRAINT user_affiliates_points_rule_values;

COMMENT ON COLUMN user_affiliates.points_rule_threshold_amount IS 'Invite points threshold captured when inviter binding succeeds';
COMMENT ON COLUMN user_affiliates.points_rule_reward_points IS 'Invite points reward captured when inviter binding succeeds';
COMMENT ON COLUMN user_affiliates.points_rule_window_days IS 'Invite qualification window captured when inviter binding succeeds';
COMMENT ON COLUMN user_affiliates.points_rule_freeze_hours IS 'Invite reward freeze duration captured when inviter binding succeeds';
COMMENT ON COLUMN user_affiliates.points_rule_version IS 'Invite points rule snapshot schema version';
