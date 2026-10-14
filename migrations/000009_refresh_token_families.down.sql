DROP INDEX IF EXISTS refresh_tokens_family_active_idx;

ALTER TABLE refresh_tokens
    DROP COLUMN IF EXISTS parent_id,
    DROP COLUMN IF EXISTS family_id;
