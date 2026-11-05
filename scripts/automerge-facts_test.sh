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
    # The second view (after collection) can report a new push via HEAD_AFTER.
    calls="$(cat "$STATE/views" 2>/dev/null || echo 0)"
    echo $((calls + 1)) >"$STATE/views"
    head=sha7
    [ "$calls" -ge 1 ] && [ -n "${HEAD_AFTER:-}" ] && head="$HEAD_AFTER"
    printf '{"number":7,"state":"OPEN","isDraft":false,"isCrossRepository":false,"headRefName":"feat/demo-integration","headRefOid":"%s","baseRefName":"%s","labels":[{"name":"overload-automation"}],"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN","reviewDecision":""}\n' "$head" "${BASE_REF:-main}" ;;
  *"rules/branches/release%231"*) echo '{"context":"build","app_id":15368}' ;;
  *"branches/release%231/protection/required_status_checks"*) echo '[]' ;;
  *"compare/release%231...sha7"*) echo 'base7' ;;
  *"release#1"*) echo "unencoded branch name in: $args" >&2; exit 1 ;;
  *"rules/branches/main"*)
    # Rules span two pages; the second page holds the overload requirement.
    case "$args" in *--paginate*) ;; *) echo "rules must be paginated: $args" >&2; exit 1 ;; esac
    echo '{"context":"build","app_id":15368}'
    echo '{"context":"overload","app_id":null}' ;;
  *"branches/main/protection/required_status_checks"*)
    if [ -n "${CLASSIC_404:-}" ]; then echo "gh: Not Found (HTTP 404)" >&2; exit 1; fi
    if [ -n "${CLASSIC_500:-}" ]; then echo "gh: Server Error (HTTP 500)" >&2; exit 1; fi
    echo '[{"context":"build","app_id":15368},{"context":"legacy-ci","app_id":null}]' ;;
  "api repos/o/r/pulls/7 "*|"api repos/o/r/pulls/7") echo "${CHANGED_FILES:-3}" ;;
  *"check-runs"*)
    echo '{"id":10,"name":"build","app_id":15368,"conclusion":"failure","started_at":"2026-01-01T00:00:00Z"}'
    echo '{"id":11,"name":"build","app_id":15368,"conclusion":"success","started_at":"2026-01-02T00:00:00Z"}'
    # Same name from another app must stay separate.
    echo '{"id":12,"name":"build","app_id":999,"conclusion":"failure","started_at":"2026-01-03T00:00:00Z"}'
    # A re-run that is still queued has no start time yet but is the newest.
    echo '{"id":20,"name":"lint","app_id":15368,"conclusion":"success","started_at":"2026-01-02T00:00:00Z"}'
    echo '{"id":21,"name":"lint","app_id":15368,"conclusion":"queued","started_at":null}' ;;
  *"commits/sha7/status"*)
    # Two pages of the combined status.
    echo '{"context":"overload","state":"success"}'
    echo '{"context":"codecov","state":"pending"}' ;;
  "api graphql"*)
    # Repository identifiers must stay strings (-f), never typed (-F).
    case "$args" in *"-F owner="*|*"-F name="*) echo "typed repository identifier: $args" >&2; exit 1 ;; esac
    if [[ "$args" == *"cursor=page2"* ]]; then
      echo '{"unresolved":1,"hasNextPage":false,"endCursor":null}'
    else
      echo '{"unresolved":0,"hasNextPage":true,"endCursor":"page2"}'
    fi ;;
  *"pulls/7/files"*)
    if [ -n "${BIG_FILES:-}" ]; then cat "$BIG_FILES"; exit 0; fi
    echo '{"path":"integrations/demo/demo.go","additions":10,"deletions":0,"status":"added","previous":null,"patch":"+package demo"}'
    echo '{"path":"cmd/server/main.go","additions":1,"deletions":0,"status":"modified","previous":null,"patch":"@@ -1 +1,2 @@\n+\tdemo.New(),"}'
    echo '{"path":"integrations/demo/README.md","additions":0,"deletions":0,"status":"renamed","previous":"docs/architecture.md","patch":null}' ;;
  *"compare/"*) echo 'base7' ;;
  *"contents/integrations?ref=base7"*) echo '["github","postgres"]' ;;
  *) echo "unexpected gh call: $args" >&2; exit 1 ;;
esac
EOF
chmod +x "$work/bin/gh"

gather() {
  rm -f "$work/views"
  PATH="$work/bin:$PATH" STATE="$work" GITHUB_REPOSITORY=o/r AUTOMERGE_POLL_SECONDS=0 bash "$ROOT/scripts/automerge-facts.sh" 7
}
facts="$(gather)" || fail "facts script failed: $facts"

[ "$(jq -c .required_checks <<<"$facts")" = '[{"context":"build","app_id":15368},{"context":"legacy-ci","app_id":null},{"context":"overload","app_id":null}]' ] ||
  fail "required checks from rulesets and classic protection: $(jq -c .required_checks <<<"$facts")"
ok "combines ruleset and classic required checks, keeping their app"
[ "$(CLASSIC_404=1 gather | jq -c .required_checks)" = '[{"context":"build","app_id":15368},{"context":"overload","app_id":null}]' ] || fail "classic protection absent"
ok "treats missing classic protection as no extra checks"
if CLASSIC_500=1 gather >/dev/null 2>&1; then fail "an unreadable classic protection must fail the facts"; fi
ok "fails when classic protection cannot be read"

[ "$(jq .files_truncated <<<"$facts")" = false ] || fail "complete list reported truncated"
[ "$(CHANGED_FILES=3001 gather | jq .files_truncated)" = true ] || fail "truncated file list not detected"
ok "detects a file list GitHub truncated"

[ "$(jq -c .labels <<<"$facts")" = '["overload-automation"]' ] || fail "labels: $(jq -c .labels <<<"$facts")"
[ "$(jq -c .check_runs <<<"$facts")" = '[{"name":"build","app_id":999,"conclusion":"failure"},{"name":"build","app_id":15368,"conclusion":"success"},{"name":"lint","app_id":15368,"conclusion":"queued"}]' ] ||
  fail "latest run per check: $(jq -c .check_runs <<<"$facts")"
ok "keeps the newest run of each check per app, even one not started yet"

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

# A large PR must not overflow the command line when facts are assembled.
big="$(jq -cn '[range(3000) | {path: ("integrations/demo/file_\(.)_" + ("x" * 400) + ".go"), additions: 1, deletions: 0, status: "added", previous: null, patch: null}] | .[]')"
printf '%s\n' "$big" >"$work/big-files"
facts="$(BIG_FILES="$work/big-files" CHANGED_FILES=3000 gather)" || fail "facts failed for a 3000-file PR"
[ "$(jq '.files | length' <<<"$facts")" = 3000 ] || fail "large PR lost files: $(jq '.files | length' <<<"$facts")"
ok "assembles facts for a 3000-file PR"

if HEAD_AFTER=sha8 gather >/dev/null 2>&1; then fail "a push during collection must fail the facts"; fi
ok "fails when the PR head moves while facts are collected"

[ "$(BASE_REF='release#1' gather | jq -c .required_checks)" = '[{"context":"build","app_id":15368}]' ] || fail "branch names must be URL-encoded"
ok "URL-encodes the base branch in API paths"

echo "PASS: automerge facts" >&2
