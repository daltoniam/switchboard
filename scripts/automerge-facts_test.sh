#!/usr/bin/env bash
# Verifies scripts/automerge-facts.sh turns GitHub API responses into the
# facts the decision needs, using a stub gh that serves canned pages.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*" >&2; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"

# The stub ignores --jq/--paginate and returns what a paginated, jq-filtered
# gh call would print, so the script's own filters are not under test here;
# the shapes and pagination handling are.
cat >"$work/bin/gh" <<'EOF'
#!/usr/bin/env bash
args="$*"
case "$args" in
  "pr view"*)
    echo '{"number":7,"state":"OPEN","isDraft":false,"isCrossRepository":false,"headRefName":"feat/demo-integration","headRefOid":"sha7","baseRefName":"main","labels":[{"name":"overload-automation"}],"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN","reviewDecision":""}' ;;
  *"rules/branches/main"*) echo '["build","overload"]' ;;
  *"check-runs"*)
    echo '{"name":"build","conclusion":"failure","started_at":"2026-01-01T00:00:00Z"}'
    echo '{"name":"build","conclusion":"success","started_at":"2026-01-02T00:00:00Z"}' ;;
  *"commits/sha7/status"*)
    # Two pages of the combined status.
    echo '{"context":"overload","state":"success"}'
    echo '{"context":"codecov","state":"pending"}' ;;
  "api graphql"*)
    if [[ "$args" == *"cursor=page2"* ]]; then
      echo '{"unresolved":1,"hasNextPage":false,"endCursor":null}'
    else
      echo '{"unresolved":0,"hasNextPage":true,"endCursor":"page2"}'
    fi ;;
  *"pulls/7/files"*)
    echo '{"path":"integrations/demo/demo.go","additions":10,"deletions":0,"status":"added","previous":null,"patch":"+package demo"}'
    echo '{"path":"cmd/server/main.go","additions":1,"deletions":0,"status":"modified","previous":null,"patch":"@@ -1 +1,2 @@\n+\tdemo.New(),"}'
    echo '{"path":"integrations/demo/README.md","additions":0,"deletions":0,"status":"renamed","previous":"docs/architecture.md","patch":null}' ;;
  *"compare/"*) echo 'base7' ;;
  *"contents/integrations?ref=base7"*) echo '["github","postgres"]' ;;
  *) echo "unexpected gh call: $args" >&2; exit 1 ;;
esac
EOF
chmod +x "$work/bin/gh"

facts="$(PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r AUTOMERGE_POLL_SECONDS=0 bash "$ROOT/scripts/automerge-facts.sh" 7)" ||
  fail "facts script failed: $facts"

[ "$(jq -c .labels <<<"$facts")" = '["overload-automation"]' ] || fail "labels: $(jq -c .labels <<<"$facts")"
[ "$(jq -c .check_runs <<<"$facts")" = '[{"name":"build","conclusion":"success"}]' ] ||
  fail "latest run per check: $(jq -c .check_runs <<<"$facts")"
ok "keeps only the latest run of each check"

[ "$(jq -c '[.statuses[].context]' <<<"$facts")" = '["overload","codecov"]' ] ||
  fail "statuses across pages: $(jq -c .statuses <<<"$facts")"
ok "collects commit statuses from every page"

[ "$(jq .unresolved_threads <<<"$facts")" = 1 ] || fail "threads across pages: $(jq .unresolved_threads <<<"$facts")"
ok "counts unresolved review threads on every page"

[ "$(jq -c '.files[] | select(.path == "docs/architecture.md") | .status' <<<"$facts")" = '"removed"' ] ||
  fail "rename source: $(jq -c .files <<<"$facts")"
ok "lists a rename's source as removed"

[ "$(jq -c '[.files[] | select(.patch != null) | .path]' <<<"$facts")" = '["cmd/server/main.go"]' ] ||
  fail "patches: $(jq -c '[.files[] | {path, patch}]' <<<"$facts")"
ok "keeps patches only for registration files"

[ "$(jq -c .base_integrations <<<"$facts")" = '["github","postgres"]' ] || fail "base integrations"
ok "lists integrations at the merge base"

echo "PASS: automerge facts" >&2
