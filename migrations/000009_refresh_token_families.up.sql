ALTER TABLE refresh_tokens
    ADD COLUMN family_id uuid,
    ADD COLUMN parent_id uuid REFERENCES refresh_tokens(id) ON DELETE SET NULL;

UPDATE refresh_tokens SET family_id = id WHERE family_id IS NULL;

ALTER TABLE refresh_tokens ALTER COLUMN family_id SET NOT NULL;

CREATE INDEX refresh_tokens_family_active_idx ON refresh_tokens (family_id)
    WHERE revoked_at IS NULL;
