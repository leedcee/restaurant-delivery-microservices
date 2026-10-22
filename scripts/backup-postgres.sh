#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

output_dir="${1:-/backups}"
: "${DATABASE_URL:?DATABASE_URL is required}"
: "${BACKUP_AGE_RECIPIENT:?BACKUP_AGE_RECIPIENT is required}"

retention_days="${BACKUP_RETENTION_DAYS:-14}"
if ! [[ "$retention_days" =~ ^[0-9]+$ ]]; then
  echo "BACKUP_RETENTION_DAYS must be a non-negative integer" >&2
  exit 2
fi

mkdir -p "$output_dir"
work_dir="$(mktemp -d)"
trap 'rm -rf -- "$work_dir"' EXIT

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
dump_name="database.dump"
archive_name="postgres-${timestamp}.tar.age"
partial_path="${output_dir}/${archive_name}.partial"
final_path="${output_dir}/${archive_name}"

pg_dump \
  --dbname="$DATABASE_URL" \
  --format=custom \
  --compress=9 \
  --no-owner \
  --no-acl \
  --file="${work_dir}/${dump_name}"

dump_sha256="$(sha256sum "${work_dir}/${dump_name}" | awk '{print $1}')"
cat >"${work_dir}/manifest.txt" <<EOF
created_at=${timestamp}
dump_file=${dump_name}
dump_sha256=${dump_sha256}
pg_dump_version=$(pg_dump --version)
format=postgres-custom
encryption=age
EOF

tar -C "$work_dir" -cf "${work_dir}/backup.tar" "$dump_name" manifest.txt
age --recipient "$BACKUP_AGE_RECIPIENT" --output "$partial_path" "${work_dir}/backup.tar"
mv -- "$partial_path" "$final_path"

if (( retention_days > 0 )); then
  find "$output_dir" -maxdepth 1 -type f -name 'postgres-*.tar.age' -mtime "+${retention_days}" -delete
fi

echo "$final_path"
