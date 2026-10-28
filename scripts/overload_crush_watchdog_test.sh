#!/usr/bin/env bash
# Verifies process-group kill used by overload-implement stops descendant processes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/overload-crush-watchdog.sh
source "${ROOT}/scripts/overload-crush-watchdog.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*" >&2; }

alive() {
  local pid=$1
  ps -o stat= -p "$pid" 2>/dev/null | grep -qv '^Z'
}

workflow="${ROOT}/.github/workflows/overload-implement.yml"
grep -q 'setsid crush run' "$workflow" || fail "workflow must start Crush with setsid"
grep -q 'overload_kill_crush_tree' "$workflow" || fail "workflow must use overload_kill_crush_tree"
ok "workflow wiring"

marker="${ROOT}/.crush-watchdog-test-$$"
setsid bash -c "sleep 9999 & touch '$marker'; exec sleep 9999" &
leader=$!
sleep 0.5
alive "$leader" || fail "test leader did not start"
[ -f "$marker" ] || fail "expected marker file from child"

overload_kill_crush_tree "$leader" TERM
for _ in $(seq 1 20); do
  alive "$leader" || break
  sleep 0.1
done
alive "$leader" && overload_kill_crush_tree "$leader" KILL
sleep 0.2
alive "$leader" && fail "leader still running after group kill"

if pgrep -f "sleep 9999" >/dev/null 2>&1; then
  pkill -f "sleep 9999" 2>/dev/null || true
  fail "sleep descendant survived process-group kill"
fi
rm -f "$marker"

ok "process-group kill stops Crush-like tree"
echo "ok: overload crush watchdog tests"
