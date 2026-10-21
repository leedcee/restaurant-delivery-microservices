#!/usr/bin/env bash
set -euo pipefail

project="restaurant-delivery-e2e"
compose_file="docker-compose.e2e.yml"
compose=(docker compose -p "$project" -f "$compose_file")
export E2E_BASE_URL="${E2E_BASE_URL:-http://127.0.0.1:18080}"
export E2E_DATABASE_URL="${E2E_DATABASE_URL:-postgres://restaurant_test:restaurant_test@127.0.0.1:55432/restaurant_delivery_test?sslmode=disable}"

wait_http() {
  local url="$1"
  local attempts="${2:-30}"
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl --fail --silent --show-error --max-time 3 "$url" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $url" >&2
  return 1
}

echo "Stopping restaurant and preparing a durable pending outbox event"
"${compose[@]}" stop demo-restaurant
E2E_RESILIENCE_PHASE=prepare-restart go test -count=1 -tags=e2e -run '^TestPrepareOutboxBeforeAPIRestart$' ./tests/e2e

echo "Restarting Platform API while the event is pending"
"${compose[@]}" restart platform-api
wait_http "$E2E_BASE_URL/health/ready"

echo "Restoring restaurant and verifying eventual delivery without duplication"
"${compose[@]}" start demo-restaurant
wait_http "http://127.0.0.1:18081/health/live"
E2E_RESILIENCE_PHASE=verify-restart go test -count=1 -tags=e2e -run '^TestVerifyOutboxAfterAPIRestart$' ./tests/e2e

echo "Pausing PostgreSQL and verifying readiness reports the dependency failure"
"${compose[@]}" pause postgres
readiness_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 12 "$E2E_BASE_URL/health/ready" || true)"
if [[ "$readiness_status" != "503" ]]; then
  "${compose[@]}" unpause postgres
  echo "readiness returned $readiness_status while PostgreSQL was paused, want 503" >&2
  exit 1
fi

echo "Restoring PostgreSQL and verifying API connection-pool recovery"
"${compose[@]}" unpause postgres
for attempt in {1..30}; do
  if "${compose[@]}" exec -T postgres pg_isready -U restaurant_test -d restaurant_delivery_test >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == "30" ]]; then
    echo "PostgreSQL did not become ready" >&2
    exit 1
  fi
  sleep 1
done
wait_http "$E2E_BASE_URL/health/ready"
E2E_RESILIENCE_PHASE=verify-database go test -count=1 -tags=e2e -run '^TestVerifyDatabaseRecovery$' ./tests/e2e
