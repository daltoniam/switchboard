#!/usr/bin/env bash
# Lifecycle contract tests for scripts/compose-dev.sh (no long-running stack).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT}/scripts/compose-dev.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*" >&2; }

[[ -x "$SCRIPT" ]] || fail "compose-dev.sh must be executable"
[[ -f "${ROOT}/docker-compose.dev.yml" ]] || fail "missing docker-compose.dev.yml"
[[ -f "${ROOT}/Dockerfile.dev" ]] || fail "missing Dockerfile.dev"
[[ -f "${ROOT}/paseo.json" ]] || fail "missing paseo.json"

help_out="$("$SCRIPT" --help)"
printf '%s\n' "$help_out" | grep -q 'check' || fail "help missing check"
printf '%s\n' "$help_out" | grep -q 'up' || fail "help missing up"
printf '%s\n' "$help_out" | grep -q 'status' || fail "help missing status"
printf '%s\n' "$help_out" | grep -q 'endpoints' || fail "help missing endpoints"
printf '%s\n' "$help_out" | grep -q 'logs' || fail "help missing logs"
printf '%s\n' "$help_out" | grep -q 'down' || fail "help missing down"
printf '%s\n' "$help_out" | grep -q 'destroy' || fail "help missing destroy"
ok "help verbs"

if "$SCRIPT" destroy >/tmp/switchboard-compose-destroy.err 2>&1; then
  fail "destroy without CONFIRM must fail"
fi
grep -q 'refusing destroy' /tmp/switchboard-compose-destroy.err || fail "destroy refusal message"
ok "destroy refused without CONFIRM"

if CONFIRM=wrong-destroy "$SCRIPT" destroy >/tmp/switchboard-compose-destroy2.err 2>&1; then
  fail "destroy with wrong CONFIRM must fail"
fi
grep -q 'refusing destroy' /tmp/switchboard-compose-destroy2.err || fail "wrong CONFIRM must refuse"
ok "destroy refused with wrong CONFIRM"

if awk '/^cmd_down\(\)/,/^}/ {print}' "$SCRIPT" | grep -vE '^[[:space:]]*#' | grep -qE 'down[[:space:]].*-v|[[:space:]]-v([[:space:]]|$)'; then
  fail "cmd_down must not pass -v"
fi
ok "down never uses -v"

if ! awk '/^cmd_destroy\(\)/,/^}/ {print}' "$SCRIPT" | grep -vE '^[[:space:]]*#' | grep -q -- 'down -v'; then
  fail "cmd_destroy must use down -v"
fi
ok "destroy uses down -v after CONFIRM"

if ! grep -q 'docker compose -p "$COMPOSE_PROJECT"' "$SCRIPT"; then
  fail "lifecycle must pass docker compose -p"
fi
ok "compose -p present"

if grep -qE 'GITHUB_TOKEN|LINEAR_API_KEY' "${ROOT}/docker-compose.dev.yml"; then
  fail "dev compose must not interpolate provider tokens"
fi
ok "dev compose has no provider tokens"

if grep -q '3847:3847' "${ROOT}/docker-compose.dev.yml"; then
  fail "dev compose must not use fixed host ports"
fi
if ! grep -q '127.0.0.1::3847' "${ROOT}/docker-compose.dev.yml"; then
  fail "dev compose must publish 127.0.0.1::3847"
fi
ok "dev publish form"

python3 - "$ROOT/paseo.json" <<'PY'
import json, sys
path = sys.argv[1]
data = json.load(open(path, encoding="utf-8"))
scripts = data.get("scripts") or {}
required = ("check", "up", "down", "status", "endpoints", "logs", "test", "ci")
missing = [k for k in required if k not in scripts]
if missing:
    print(f"FAIL: paseo.json missing {missing}", file=sys.stderr)
    sys.exit(1)
PY
ok "paseo.json scripts"

echo "ok: compose-dev lifecycle tests"
