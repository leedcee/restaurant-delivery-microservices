#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

backup_path="${1:?usage: restore-postgres.sh BACKUP.tar.age}"
: "${RESTORE_DATABASE_URL:?RESTORE_DATABASE_URL is required}"
: "${RESTORE_AGE_IDENTITY_FILE:?RESTORE_AGE_IDENTITY_FILE is required}"
: "${ALLOW_DATABASE_RESTORE:?set ALLOW_DATABASE_RESTORE=true after confirming the target is an empty database}"

if [[ "$ALLOW_DATABASE_RESTORE" != "true" ]]; then
  echo "Refusing restore: ALLOW_DATABASE_RESTORE must equal true" >&2
  exit 2
fi
if [[ ! -f "$backup_path" ]]; then
  echo "Backup does not exist: $backup_path" >&2
  exit 2
fi

work_dir="$(mktemp -d)"
trap 'rm -rf -- "$work_dir"' EXIT

age --decrypt --identity "$RESTORE_AGE_IDENTITY_FILE" --output "${work_dir}/backup.tar" "$backup_path"
tar -C "$work_dir" -xf "${work_dir}/backup.tar"

expected_sha256="$(awk -F= '$1 == "dump_sha256" {print $2}' "${work_dir}/manifest.txt")"
actual_sha256="$(sha256sum "${work_dir}/database.dump" | awk '{print $1}')"
if [[ -z "$expected_sha256" || "$actual_sha256" != "$expected_sha256" ]]; then
  echo "Backup checksum verification failed" >&2
  exit 1
fi

pg_restore --list "${work_dir}/database.dump" >/dev/null
pg_restore \
  --dbname="$RESTORE_DATABASE_URL" \
  --exit-on-error \
  --single-transaction \
  --no-owner \
  --no-acl \
  "${work_dir}/database.dump"
psql "$RESTORE_DATABASE_URL" -v ON_ERROR_STOP=1 -c 'SELECT 1' >/dev/null

echo "Restore completed and verified: $backup_path"
