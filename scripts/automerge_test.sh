#!/usr/bin/env bash
# Verifies what scripts/automerge.sh does with each decision, using a stub gh
# and stub facts so no GitHub state is touched.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*" >&2; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/facts"
log="$work/gh.log"

# Stub gh: logs every call, lists PRs from $work/prs, and returns existing
# comment bodies from $work/comments-<pr>.
cat >"$work/bin/gh" <<EOF
#!/usr/bin/env bash
printf 'GH_TOKEN=%s gh %s\n' "\${GH_TOKEN:-}" "\$*" >>"$log"
case "\$*" in
  "pr list"*) cat "$work/prs" ;;
  *"repos/o/r/issues/"*"/comments"*)
    args="\$*"
    number="\${args#*repos/o/r/issues/}"
    cat "$work/comments-\${number%%/*}" 2>/dev/null || true ;;
esac
exit 0
EOF
chmod +x "$work/bin/gh"

# Stub facts: the decision comes from $work/facts/<pr>.json.
cat >"$work/facts.sh" <<EOF
#!/usr/bin/env bash
cat "$work/facts/\$1.json"
EOF
cat >"$work/decide.sh" <<'EOF'
#!/usr/bin/env bash
jq '{decision: .want, integration: "keycloak", reasons: (.why // [])}'
EOF

facts() { printf '{"want":"%s","head_sha":"%s","why":%s}\n' "$2" "$3" "${4:-[]}" >"$work/facts/$1.json"; }

run() {
  : >"$log"
  PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r GH_TOKEN=read-token MERGE_TOKEN=merge-token \
    AUTOMERGE_FACTS="$work/facts.sh" AUTOMERGE_DECIDE="$work/decide.sh" \
    AUTOMERGE_MODE="$1" bash "$ROOT/scripts/automerge.sh" >"$work/out" 2>&1 ||
    fail "automerge.sh exited $? in $1 mode: $(cat "$work/out")"
}

calls() { grep -cF -- "$1" "$log" || true; }

printf '359\n326\n' >"$work/prs"
facts 359 merge sha359
facts 326 hold sha326 '["waiting for live-integration"]'

run dry-run
[ "$(calls 'pr merge')" = 0 ] || fail "dry run merged: $(cat "$log")"
[ "$(calls 'pr edit')" = 0 ] || fail "dry run labeled: $(cat "$log")"
[ "$(calls 'pr comment 359')" = 1 ] || fail "dry run did not comment on the mergeable PR: $(cat "$log")"
[ "$(calls 'pr comment 326')" = 0 ] || fail "dry run commented on a held PR"
grep -q "waiting for live-integration" "$work/out" || fail "hold reasons were not reported"
ok "dry run comments once on a mergeable PR and never merges"

printf '<!-- automerge:sha359 -->\n' >"$work/comments-359"
run dry-run
[ "$(calls 'pr comment 359')" = 0 ] || fail "dry run commented twice on the same head"
ok "dry run does not repeat itself for the same head"

facts 359 merge sha359-new
run dry-run
[ "$(calls 'pr comment 359')" = 1 ] || fail "dry run did not comment on a new head"
ok "dry run comments again after a new push"

rm -f "$work/comments-359"
run merge
grep -qF 'gh pr edit 359 --repo o/r --add-label release:skip' "$log" || fail "merge did not add release:skip: $(cat "$log")"
grep -qF 'GH_TOKEN=merge-token gh pr merge 359 --repo o/r --squash --match-head-commit sha359-new' "$log" ||
  fail "merge did not squash at the decided head with the merge token: $(cat "$log")"
label_line="$(grep -nF 'pr edit 359' "$log" | cut -d: -f1)"
merge_line="$(grep -nF 'pr merge 359' "$log" | cut -d: -f1)"
[ "$label_line" -lt "$merge_line" ] || fail "release:skip must be added before merging"
[ "$(calls 'pr merge 326')" = 0 ] || fail "merge mode merged a held PR"
ok "merge mode labels release:skip, then merges at the decided head"

run disabled
[ "$(calls 'pr ')" = 0 ] || fail "disabled mode touched PRs: $(cat "$log")"
ok "disabled mode does nothing"

: >"$log"
if PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r AUTOMERGE_FACTS="$work/facts.sh" \
  AUTOMERGE_DECIDE="$work/decide.sh" AUTOMERGE_MODE=yolo bash "$ROOT/scripts/automerge.sh" >/dev/null 2>&1; then
  fail "unknown mode was accepted"
fi
ok "unknown mode is rejected"

echo "PASS: automerge runner" >&2
