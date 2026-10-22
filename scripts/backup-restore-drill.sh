#!/usr/bin/env bash
set -Eeuo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${RESTORE_DATABASE_URL:?RESTORE_DATABASE_URL is required}"
: "${RESTORE_DATABASE_NAME:?RESTORE_DATABASE_NAME is required}"

work_dir="$(mktemp -d)"
trap 'dropdb --if-exists --maintenance-db="$DATABASE_URL" "$RESTORE_DATABASE_NAME" >/dev/null 2>&1 || true; rm -rf -- "$work_dir"' EXIT

age-keygen --output "${work_dir}/identity.txt" >/dev/null
export BACKUP_AGE_RECIPIENT="$(age-keygen -y "${work_dir}/identity.txt")"
export BACKUP_RETENTION_DAYS=1

source_counts="$(psql "$DATABASE_URL" -At -v ON_ERROR_STOP=1 -c "SELECT (SELECT count(*) FROM restaurants) || ':' || (SELECT count(*) FROM menu_items) || ':' || (SELECT count(*) FROM orders) || ':' || (SELECT count(*) FROM order_events);")"
backup_path="$(backup-postgres.sh "${work_dir}/backups")"

dropdb --if-exists --maintenance-db="$DATABASE_URL" "$RESTORE_DATABASE_NAME"
createdb --maintenance-db="$DATABASE_URL" "$RESTORE_DATABASE_NAME"

export RESTORE_AGE_IDENTITY_FILE="${work_dir}/identity.txt"
export ALLOW_DATABASE_RESTORE=true
restore-postgres.sh "$backup_path"

restored_counts="$(psql "$RESTORE_DATABASE_URL" -At -v ON_ERROR_STOP=1 -c "SELECT (SELECT count(*) FROM restaurants) || ':' || (SELECT count(*) FROM menu_items) || ':' || (SELECT count(*) FROM orders) || ':' || (SELECT count(*) FROM order_events);")"
if [[ "$source_counts" != "$restored_counts" ]]; then
  echo "Restore drill failed: source counts $source_counts, restored counts $restored_counts" >&2
  exit 1
fi

schema_version="$(psql "$RESTORE_DATABASE_URL" -At -v ON_ERROR_STOP=1 -c 'SELECT version FROM schema_migrations WHERE dirty = false ORDER BY version DESC LIMIT 1;')"
if [[ -z "$schema_version" ]]; then
  echo "Restore drill failed: no clean schema migration version" >&2
  exit 1
fi

echo "Backup/restore drill passed: counts=$restored_counts schema_version=$schema_version"
