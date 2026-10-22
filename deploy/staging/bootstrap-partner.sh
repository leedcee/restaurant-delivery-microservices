#!/bin/sh
set -eu

database_url="$(cat "${DATABASE_URL_FILE:?DATABASE_URL_FILE is required}")"
partner_secret="$(cat "${PARTNER_SECRET_FILE:?PARTNER_SECRET_FILE is required}")"
restaurant_id="${RESTAURANT_ID:?RESTAURANT_ID is required}"
credential_name="${CREDENTIAL_NAME:-Staging demo owner}"

psql "$database_url" -v ON_ERROR_STOP=1 \
  -v restaurant_id="$restaurant_id" \
  -v partner_secret="$partner_secret" \
  -v credential_name="$credential_name" <<'SQL'
BEGIN;

WITH credential AS (
    INSERT INTO partner_credentials (restaurant_id, name, secret_hash, role)
    VALUES (
        :'restaurant_id'::uuid,
        :'credential_name',
        encode(digest(:'partner_secret', 'sha256'), 'hex'),
        'owner'
    )
    ON CONFLICT (secret_hash) DO UPDATE SET
        restaurant_id = EXCLUDED.restaurant_id,
        name = EXCLUDED.name,
        role = 'owner',
        expires_at = NULL,
        revoked_at = NULL
    RETURNING id, restaurant_id
)
INSERT INTO partner_audit_log (restaurant_id, credential_id, action, resource_type, resource_id, reason, metadata)
SELECT restaurant_id, id, 'credential.bootstrapped', 'partner_credential', id::text,
       'staging release bootstrap', jsonb_build_object('name', :'credential_name')
FROM credential;

UPDATE partner_credentials
SET revoked_at = now()
WHERE restaurant_id = :'restaurant_id'::uuid
  AND name = 'Legacy owner key'
  AND secret_hash <> encode(digest(:'partner_secret', 'sha256'), 'hex')
  AND revoked_at IS NULL;

COMMIT;
SQL
