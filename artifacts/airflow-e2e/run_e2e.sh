#!/usr/bin/env bash
# Reproducible Switchboard ↔ Airflow 3 native REST E2E (see artifacts/MERGE_NOTES.md).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
E2E_DIR="$(cd "$(dirname "$0")" && pwd)"
LOG="${ROOT}/artifacts/airflow_e2e_proof.log"

exec > >(tee -a "$LOG") 2>&1

echo "=== Switchboard Airflow E2E $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
echo "branch: $(git -C "$ROOT" branch --show-current)"
echo "commit: $(git -C "$ROOT" rev-parse --short HEAD)"

cd "$E2E_DIR"
chmod 666 ./config/simple_auth_manager_passwords.json 2>/dev/null || true
if ! curl -sf http://localhost:8080/api/v2/version >/dev/null; then
  echo "Starting Airflow sandbox via docker compose..."
  sudo docker compose up -d
  for _ in $(seq 1 36); do
    curl -sf http://localhost:8080/api/v2/version >/dev/null && break
    sleep 10
  done
fi
curl -s http://localhost:8080/api/v2/version | jq .

export AIRFLOW_BASE_URL="${AIRFLOW_BASE_URL:-http://127.0.0.1:8080}"
export AIRFLOW_USERNAME="${AIRFLOW_USERNAME:-admin}"
export AIRFLOW_PASSWORD="${AIRFLOW_PASSWORD:-switchboard-e2e}"

echo "--- POST /auth/token (same path as adapter) ---"
curl -s -X POST "${AIRFLOW_BASE_URL}/auth/token" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${AIRFLOW_USERNAME}\",\"password\":\"${AIRFLOW_PASSWORD}\"}" | jq -c '{access_token: (.access_token | .[0:24] // null)}'

echo "--- Switchboard adapter tools (integrations/airflow) ---"
cd "$ROOT"
go run ./artifacts/airflow-e2e/run_proof.go

echo "=== E2E complete; log at $LOG ==="
