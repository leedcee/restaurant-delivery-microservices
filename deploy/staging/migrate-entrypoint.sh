#!/bin/sh
set -eu

database_url="$(cat "${DATABASE_URL_FILE:?DATABASE_URL_FILE is required}")"
exec /migrate -path=/migrations -database="$database_url" "$@"
