CREATE TABLE partner_credentials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id uuid NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    secret_hash text NOT NULL UNIQUE,
    role text NOT NULL CHECK (role IN ('owner', 'manager', 'kitchen', 'operator')),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    revoked_at timestamptz,
    last_used_at timestamptz,
    CHECK (expires_at IS NULL OR expires_at > created_at)
);

CREATE INDEX partner_credentials_restaurant_idx
    ON partner_credentials (restaurant_id, created_at DESC);

CREATE INDEX partner_credentials_active_idx
    ON partner_credentials (secret_hash)
    WHERE revoked_at IS NULL;

INSERT INTO partner_credentials (restaurant_id, name, secret_hash, role)
SELECT restaurant_id, 'Legacy owner key', api_key_hash, 'owner'
FROM restaurant_integrations
ON CONFLICT (secret_hash) DO NOTHING;

CREATE TABLE partner_audit_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    restaurant_id uuid NOT NULL REFERENCES restaurants(id) ON DELETE RESTRICT,
    credential_id uuid REFERENCES partner_credentials(id) ON DELETE SET NULL,
    action text NOT NULL CHECK (char_length(action) BETWEEN 1 AND 120),
    resource_type text NOT NULL CHECK (char_length(resource_type) BETWEEN 1 AND 120),
    resource_id text,
    request_id text,
    trace_id text,
    reason text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX partner_audit_log_restaurant_time_idx
    ON partner_audit_log (restaurant_id, created_at DESC);

CREATE OR REPLACE FUNCTION reject_partner_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'partner audit log is append-only';
END;
$$;

CREATE TRIGGER partner_audit_log_append_only
    BEFORE UPDATE OR DELETE ON partner_audit_log
    FOR EACH ROW EXECUTE FUNCTION reject_partner_audit_mutation();
