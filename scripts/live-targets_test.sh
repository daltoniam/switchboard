#!/usr/bin/env bash
# Verifies which integrations the live-integration job tests for a change.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT}/scripts/live-targets.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*" >&2; }

repo="$(mktemp -d)"
trap 'rm -rf "$repo"' EXIT
git -C "$repo" init -q -b main
git -C "$repo" config user.email test@example.test
git -C "$repo" config user.name test
git -C "$repo" config commit.gpgsign false

write() { mkdir -p "$(dirname "$repo/$1")"; printf '%s\n' "${2:-x}" > "$repo/$1"; }
commit() { git -C "$repo" add -A && git -C "$repo" commit -q -m "$1" && git -C "$repo" rev-parse HEAD; }

write integrations/postgres/postgres.go
write integrations/github/github.go
write web/live_test.go
write web/live_postgres_test.go
write web/ui_test.go
write README.md
base="$(commit base)"

# expect <description> <expected exit> <expected output> -- runs against HEAD
expect() {
  local description=$1 want_status=$2 want_output=$3 output status=0
  output="$(cd "$repo" && bash "$SCRIPT" "$base" HEAD 2>&1)" || status=$?
  [ "$status" = "$want_status" ] || fail "$description: exit $status, want $want_status; output: $output"
  [ "$output" = "$want_output" ] || fail "$description: output [$output], want [$want_output]"
  ok "$description"
}

reset() { git -C "$repo" reset -q --hard "$base"; }

write README.md changed
commit docs >/dev/null
expect "docs-only change tests nothing" 0 ""
reset

write integrations/postgres/queries.go
commit pg >/dev/null
expect "changed integration with a spec is tested" 0 "postgres"
reset

write integrations/github/issues.go
commit gh >/dev/null
expect "changed existing integration without a spec is skipped" 0 ""
reset

write web/live_postgres_test.go changed
commit spec >/dev/null
expect "changed spec is tested" 0 "postgres"
reset

write integrations/keycloak/keycloak.go
commit kc >/dev/null
expect "new integration without a spec fails" 1 "new integration keycloak has no live spec; add web/live_keycloak_test.go (see docs/integration-ui-testing.md)"
reset

write integrations/keycloak/keycloak.go
write web/live_keycloak_test.go
commit kc >/dev/null
expect "new integration with a spec is tested" 0 "keycloak"
reset

write web/live_test.go changed
commit harness >/dev/null
expect "harness change tests every spec" 0 "postgres"
reset

write mcp.go
commit core >/dev/null
expect "core change tests every spec" 0 "postgres"
reset

write server/server.go
write integrations/postgres/tools.go
write web/live_keycloak_test.go
write integrations/keycloak/keycloak.go
commit mixed >/dev/null
expect "targets are unique and sorted" 0 $'keycloak\npostgres'
reset

git -C "$repo" rm -q web/live_postgres_test.go
commit drop >/dev/null
expect "removed spec is not tested" 0 ""
reset

git -C "$repo" mv integrations/postgres/postgres.go integrations/github/postgres.go
commit move >/dev/null
expect "moving a file out of an integration tests the source" 0 "postgres"
reset

# The base branch deletes github after the head branched; the head still
# edits it. Existence must be judged at the merge base, like the diff.
write integrations/github/issues.go
head_sha="$(commit edit-github)"
git -C "$repo" checkout -q -b later "$base"
git -C "$repo" rm -q -r integrations/github
later="$(commit drop-github)"
git -C "$repo" checkout -q main
output="$(cd "$repo" && bash "$SCRIPT" "$later" "$head_sha" 2>&1)" ||
  fail "integration deleted on base after branching: exit $?, output: $output"
[ -z "$output" ] || fail "integration deleted on base after branching: output [$output], want []"
ok "existing integration is judged at the merge base"
reset

echo "PASS: live targets" >&2
