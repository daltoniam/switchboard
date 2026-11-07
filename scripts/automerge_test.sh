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
  *"issues/404/comments"*) exit 1 ;;
  "pr merge 409"*) exit 1 ;;
  "pr edit 410 --repo o/r --add-label release:skip"*)
    # The label does not exist until it is created.
    [ -f "$work/label-created" ] || { echo "'release:skip' not found" >&2; exit 1; } ;;
  "label create release:skip"*) touch "$work/label-created" ;;
  "pr edit 411 --repo o/r --add-label release:skip"*) exit 1 ;;
  "pr list"*) cat "$work/prs" ;;
  "run list --workflow overload-implement.yml"*)
    # Only an explicit status filter sees an old run that is still going.
    case "\$*" in *"--status in_progress"*|*"--status queued"*|*"--status waiting"*|*"--status pending"*|*"--status requested"*) ;; *) echo 0; exit 0 ;; esac
    case "\$*" in *"--status in_progress"*) cat "$work/impl-runs" 2>/dev/null || echo 0 ;; *) echo 0 ;; esac ;;
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
jq '{decision: (if .want == "merge" then "merge" else "hold" end), next: (.next // .want), integration: "keycloak", reasons: (.why // [])}'
EOF

facts() { printf '{"want":"%s","head_sha":"%s","why":%s,"labels":%s}\n' "$2" "$3" "${4:-[]}" "${5:-[]}" >"$work/facts/$1.json"; }
# next_facts <pr> <next> <sha> <reasons-json>: a held PR with an explicit next step.
next_facts() { printf '{"want":"hold","next":"%s","head_sha":"%s","why":%s,"labels":[]}\n' "$2" "$3" "$4" >"$work/facts/$1.json"; }

run() {
  : >"$log"
  PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r GH_TOKEN=read-token MERGE_TOKEN=merge-token \
    AUTOMERGE_FACTS="$work/facts.sh" AUTOMERGE_DECIDE="$work/decide.sh" \
    AUTOMERGE_MODE="$1" AUTOFIX="${AUTOFIX:-on}" bash "$ROOT/scripts/automerge.sh" >"$work/out" 2>&1 ||
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
grep -q 'gh pr list .*--limit 1000' "$log" || fail "PR discovery must not stop at the default page size: $(grep 'pr list' "$log")"
ok "discovers every open automation PR, not just the first page"

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

printf '404\n' >"$work/prs"
facts 404 merge sha404
: >"$log"
if PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r GH_TOKEN=read-token AUTOMERGE_FACTS="$work/facts.sh" \
  AUTOMERGE_DECIDE="$work/decide.sh" AUTOMERGE_MODE=dry-run bash "$ROOT/scripts/automerge.sh" >"$work/out" 2>&1; then
  fail "a failed comment lookup should fail the run"
fi
[ "$(calls 'pr comment 404')" = 0 ] || fail "dry run commented although it could not read existing comments"
ok "dry run does not comment when existing comments cannot be read"

printf '409\n' >"$work/prs"
facts 409 merge sha409
: >"$log"
if PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r GH_TOKEN=read-token AUTOMERGE_FACTS="$work/facts.sh" \
  AUTOMERGE_DECIDE="$work/decide.sh" AUTOMERGE_MODE=merge bash "$ROOT/scripts/automerge.sh" >"$work/out" 2>&1; then
  fail "a failed merge should fail the run"
fi
grep -qF 'gh pr edit 409 --repo o/r --remove-label release:skip' "$log" || fail "failed merge left release:skip: $(cat "$log")"
ok "a failed merge removes the release:skip label it added"

facts 409 merge sha409 '[]' '["release:skip"]'
: >"$log"
PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r GH_TOKEN=read-token AUTOMERGE_FACTS="$work/facts.sh" \
  AUTOMERGE_DECIDE="$work/decide.sh" AUTOMERGE_MODE=merge bash "$ROOT/scripts/automerge.sh" >/dev/null 2>&1 || true
[ "$(calls 'remove-label')" = 0 ] || fail "removed a release:skip label a person had set"
[ "$(calls 'add-label')" = 0 ] || fail "re-added a label that was already there"
ok "a failed merge keeps a release:skip label that was already there"

# --- Missing release:skip label ---
rm -f "$work/label-created"
printf '410\n411\n412\n' >"$work/prs"
facts 410 merge sha410
facts 411 merge sha411
facts 412 merge sha412
: >"$log"
PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r GH_TOKEN=read-token AUTOMERGE_FACTS="$work/facts.sh" \
  AUTOMERGE_DECIDE="$work/decide.sh" AUTOMERGE_MODE=merge AUTOFIX=off bash "$ROOT/scripts/automerge.sh" >"$work/out" 2>&1 && \
  fail "a PR that could not be labeled should fail the run"
grep -qF 'gh label create release:skip' "$log" || fail "missing label was not created: $(cat "$log")"
grep -qF 'pr merge 410' "$log" || fail "did not merge after creating the label: $(cat "$log")"
[ "$(calls 'pr merge 411')" = 0 ] || fail "merged a PR it could not label release:skip"
grep -qF 'pr merge 412' "$log" || fail "one labeling failure stopped the other PRs: $(cat "$log")"
ok "creates a missing release:skip label, and a PR it cannot label is skipped, not the run"

# --- Auto-fix: send the implementation job to PRs it can fix ---
rm -f "$work"/comments-*
printf '326\n346\n363\n' >"$work/prs"
next_facts 326 fix sha326 '["not mergeable (CONFLICTING)", "merge state is DIRTY"]'
next_facts 346 person sha346 '["go.mod is outside what an integration PR may change"]'
next_facts 363 fix sha363 '["overload is failure"]'
echo 0 >"$work/impl-runs"

run dry-run
[ "$(calls 'workflow run overload-implement.yml')" = 1 ] || fail "expected one fix dispatch: $(cat "$log")"
grep -qF 'gh workflow run overload-implement.yml --repo o/r -f mode=shepherd -f target=326' "$log" ||
  fail "did not send the job to the first fixable PR: $(cat "$log")"
grep -qF 'pr comment 326' "$log" || fail "did not record the fix request on the PR"
[ "$(calls 'pr comment 346')" = 0 ] || fail "commented on a PR that needs a person"
grep -q "#346 (keycloak) needs a person" "$work/out" || fail "person hold was not reported: $(cat "$work/out")"
ok "sends the implementation job to one fixable PR per run, never to a person's PR"

grep -o 'autofix:sha326:[0-9]*' "$log" | head -1 >"$work/marker326" || true
[ -s "$work/marker326" ] || fail "fix request comment has no marker"
printf '<!-- %s -->\n' "$(cat "$work/marker326")" >"$work/comments-326"
run dry-run
grep -qF -- '-f target=363' "$log" || fail "did not move on to the next fixable PR: $(cat "$log")"
[ "$(calls 'target=326')" = 0 ] || fail "asked again for the same head and findings"
ok "asks once per head and set of findings, then moves to the next PR"

printf '<!-- autofix:sha363:x -->\n' >"$work/comments-363"
next_facts 326 fix sha326 '["build is failure"]'
run dry-run
grep -qF -- '-f target=326' "$log" || fail "new findings on the same head were not sent: $(cat "$log")"
ok "asks again when the findings on a head change"

echo 1 >"$work/impl-runs"
rm -f "$work"/comments-*
run dry-run
[ "$(calls 'workflow run')" = 0 ] || fail "dispatched while the implementation job was busy"
grep -q "implementation job is busy" "$work/out" || fail "busy job was not reported"
ok "waits while an implementation run is queued or running"

echo 0 >"$work/impl-runs"
AUTOFIX=off run dry-run
[ "$(calls 'workflow run')" = 0 ] || fail "dispatched with AUTOFIX=off"
ok "AUTOFIX=off turns auto-fix off"

next_facts 326 wait sha326 '["waiting for test"]'
next_facts 363 wait sha363 '["waiting for overload"]'
run dry-run
[ "$(calls 'workflow run')" = 0 ] || fail "dispatched for PRs that are only waiting"
ok "does not send the job to PRs that are only waiting"

run disabled
[ "$(calls 'pr ')" = 0 ] || fail "disabled mode touched PRs: $(cat "$log")"
[ "$(calls 'workflow run')" = 0 ] || fail "disabled mode dispatched"
ok "disabled mode does nothing"

: >"$log"
if PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r AUTOMERGE_FACTS="$work/facts.sh" \
  AUTOMERGE_DECIDE="$work/decide.sh" AUTOMERGE_MODE=yolo bash "$ROOT/scripts/automerge.sh" >/dev/null 2>&1; then
  fail "unknown mode was accepted"
fi
ok "unknown mode is rejected"

echo "PASS: automerge runner" >&2
