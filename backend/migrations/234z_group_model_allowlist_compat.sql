-- Keep v0.2.1 and v0.2.4 group queries compatible during blue-green rollout.
-- Must run before 235; published migrations remain unchanged.
SET LOCAL lock_timeout = '5s';

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS models_list_config JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS model_allowlist JSONB NOT NULL DEFAULT '{}';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM groups
        WHERE models_list_config <> '{}'::jsonb
          AND model_allowlist <> '{}'::jsonb
          AND models_list_config IS DISTINCT FROM model_allowlist
    ) THEN
        RAISE EXCEPTION 'Conflicting legacy and current group model configurations';
    END IF;
END
$$;

UPDATE groups SET model_allowlist = models_list_config
WHERE model_allowlist = '{}'::jsonb AND models_list_config <> '{}'::jsonb;
UPDATE groups SET models_list_config = model_allowlist
WHERE models_list_config IS DISTINCT FROM model_allowlist;

CREATE OR REPLACE FUNCTION sync_group_model_allowlist_compat()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.models_list_config = '{}'::jsonb THEN
            NEW.models_list_config := NEW.model_allowlist;
        ELSIF NEW.model_allowlist = '{}'::jsonb THEN
            NEW.model_allowlist := NEW.models_list_config;
        ELSIF NEW.models_list_config IS DISTINCT FROM NEW.model_allowlist THEN
            RAISE EXCEPTION 'Conflicting legacy and current group model configurations';
        END IF;
    ELSIF NEW.models_list_config IS DISTINCT FROM OLD.models_list_config
          AND NEW.model_allowlist IS DISTINCT FROM OLD.model_allowlist THEN
        IF NEW.models_list_config IS DISTINCT FROM NEW.model_allowlist THEN
            RAISE EXCEPTION 'Conflicting legacy and current group model configurations';
        END IF;
    ELSIF NEW.models_list_config IS DISTINCT FROM OLD.models_list_config THEN
        NEW.model_allowlist := NEW.models_list_config;
    ELSIF NEW.model_allowlist IS DISTINCT FROM OLD.model_allowlist THEN
        NEW.models_list_config := NEW.model_allowlist;
    END IF;
    RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS group_model_allowlist_compat ON groups;
CREATE TRIGGER group_model_allowlist_compat
BEFORE INSERT OR UPDATE OF models_list_config, model_allowlist ON groups
FOR EACH ROW EXECUTE FUNCTION sync_group_model_allowlist_compat();
