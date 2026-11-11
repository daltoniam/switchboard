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

child_running() {
  local token=$1
  pgrep -af "$token" >/dev/null 2>&1
}

workflow="${ROOT}/.github/workflows/overload-implement.yml"
grep -q 'setsid nice -n 10 crush run' "$workflow" || fail "workflow must start Crush with setsid at lower CPU priority"
grep -q 'overload_resource_sample' "$workflow" || fail "workflow must log resource samples while Crush runs"
grep -q 'name: Make room on the runner' "$workflow" || fail "workflow must free disk and add swap before Crush runs"
grep -q 'crush_pgid=' "$workflow" || fail "workflow must capture crush_pgid at launch"
grep -q 'overload_kill_crush_tree' "$workflow" || fail "workflow must use overload_kill_crush_tree"
ok "workflow wiring"

sample_file="$(mktemp)"
OVERLOAD_PROC_ROOT="${ROOT}/scripts/testdata/proc" overload_resource_sample "/" >"$sample_file"
sample="$(cat "$sample_file")"; rm -f "$sample_file"
for field in load= mem_avail_mb= swap_used_mb= disk_free_gb=; do
  grep -q "$field" <<<"$sample" || fail "resource sample lacks $field: $sample"
done
grep -q "mem_avail_mb=6144" <<<"$sample" || fail "resource sample misread MemAvailable: $sample"
grep -q "swap_used_mb=1024" <<<"$sample" || fail "resource sample misread swap: $sample"
grep -q "load=3.50" <<<"$sample" || fail "resource sample misread load: $sample"
ok "resource sample"

# The watchdog runs on the Linux Actions runner. macOS has no setsid, so the
# process-group checks only run where the watchdog itself can.
if ! command -v setsid >/dev/null 2>&1; then
  ok "skipping process-group checks: setsid is not installed"
  exit 0
fi

tag="overload-watchdog-$$-a"
marker="${ROOT}/.${tag}"
setsid bash -c "sleep 9999 & touch '$marker'; exec tail -f /dev/null # ${tag}" &
leader=$!
sleep 0.5
pgid=$(overload_crush_pgid "$leader")
[ -n "$pgid" ] || fail "expected PGID at launch"
alive "$leader" || fail "test leader did not start"
[ -f "$marker" ] || fail "expected marker file from child"

overload_kill_crush_tree "$leader" TERM "$pgid"
for _ in $(seq 1 20); do
  alive "$leader" || break
  sleep 0.1
done
alive "$leader" && overload_kill_crush_tree "$leader" KILL "$pgid"
sleep 0.2
alive "$leader" && fail "leader still running after group kill"
child_running "$tag" && fail "descendant survived process-group kill"
rm -f "$marker"
ok "process-group kill stops Crush-like tree"

# Cleanup must use the PGID captured at launch, not a fresh lookup on a stale PID.
tag="overload-watchdog-$$-b"
setsid bash -c "sleep 9999 # ${tag} & exec tail -f /dev/null # ${tag}-lead" &
leader=$!
sleep 0.5
pgid=$(overload_crush_pgid "$leader")
[ -n "$pgid" ] || fail "expected PGID at launch"
overload_kill_crush_tree "" TERM "$pgid"
for _ in $(seq 1 20); do
  alive "$leader" || break
  sleep 0.1
done
alive "$leader" && overload_kill_crush_tree "" KILL "$pgid"
sleep 0.2
child_running "$tag" && fail "tree survived pgid-only kill"
ok "stored PGID stops tree without leader PID lookup"

echo "ok: overload crush watchdog tests"
