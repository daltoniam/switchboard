#!/usr/bin/env bash
# Gathers the facts scripts/automerge-decide.sh needs about one PR, as JSON.
# Read-only: it only calls GitHub read APIs.
#
# Usage: scripts/automerge-facts.sh <pr-number>
set -euo pipefail

pr=${1:?usage: automerge-facts.sh <pr-number>}
repo=${GITHUB_REPOSITORY:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}
owner=${repo%%/*}
name=${repo#*/}

view="$(gh pr view "$pr" --repo "$repo" --json \
  number,state,isDraft,isCrossRepository,headRefName,headRefOid,baseRefName,labels,mergeable,mergeStateStatus,reviewDecision)"
# GitHub computes mergeability lazily after main moves; asking starts the
# computation, so poll briefly instead of reporting UNKNOWN.
for _ in 1 2 3 4 5 6; do
  [ "$(jq -r .state <<<"$view")" = OPEN ] && [ "$(jq -r .mergeable <<<"$view")" = UNKNOWN ] || break
  sleep "${AUTOMERGE_POLL_SECONDS:-5}"
  view="$(gh pr view "$pr" --repo "$repo" --json \
    number,state,isDraft,isCrossRepository,headRefName,headRefOid,baseRefName,labels,mergeable,mergeStateStatus,reviewDecision)"
done
head_sha="$(jq -r .headRefOid <<<"$view")"
base_ref="$(jq -r .baseRefName <<<"$view")"
# Branch names can hold characters that mean something in a URL (#, ?, %).
base_path="$(jq -rn --arg ref "$base_ref" '$ref | @uri')"

facts_dir="$(mktemp -d)"
trap 'rm -rf "$facts_dir"' EXIT

ruleset_checks="$(gh api "repos/$repo/rules/branches/$base_path" \
  --jq '[.[] | select(.type == "required_status_checks") | .parameters.required_status_checks[]
         | {context, app_id: (.integration_id // null)}]')"
# Classic branch protection can require checks too. No protection is a 404;
# any other failure stops here rather than under-reporting requirements.
if ! classic_checks="$(gh api "repos/$repo/branches/$base_path/protection/required_status_checks" \
  --jq '[(.contexts[]? | {context: ., app_id: null}),
         (.checks[]? | {context, app_id: (if (.app_id // -1) == -1 then null else .app_id end)})]' \
  2>"$facts_dir/classic.err")"; then
  if grep -q "HTTP 404" "$facts_dir/classic.err"; then
    classic_checks='[]'
  else
    cat "$facts_dir/classic.err" >&2
    exit 1
  fi
fi
# A requirement is a context plus, optionally, the app that must report it.
required="$(jq -cn --argjson a "$ruleset_checks" --argjson b "$classic_checks" '$a + $b | unique_by([.context, .app_id])')"

# Newest run per check name and app (by ID: a queued re-run has no start time yet);
# a pending run has no conclusion yet, so report its status instead.
check_runs="$(gh api --paginate "repos/$repo/commits/$head_sha/check-runs?per_page=100" \
  --jq '.check_runs[] | {id, name, app_id: .app.id, conclusion: (.conclusion // .status)}' |
  jq -s 'group_by([.name, .app_id]) | map(sort_by(.id) | last | {name, app_id, conclusion})')"

# The combined status keeps only the latest state per context, but is paged.
statuses="$(gh api --paginate "repos/$repo/commits/$head_sha/status" --jq '.statuses[] | {context, state}' | jq -s .)"

unresolved=0
cursor=null
while :; do
  page="$(gh api graphql -F owner="$owner" -F name="$name" -F pr="$pr" -F cursor="$cursor" -f query='
    query($owner: String!, $name: String!, $pr: Int!, $cursor: String) {
      repository(owner: $owner, name: $name) {
        pullRequest(number: $pr) {
          reviewThreads(first: 100, after: $cursor) {
            nodes { isResolved }
            pageInfo { hasNextPage endCursor }
          }
        }
      }
    }' --jq '.data.repository.pullRequest.reviewThreads
      | {unresolved: ([.nodes[] | select(.isResolved | not)] | length),
         hasNextPage: .pageInfo.hasNextPage, endCursor: .pageInfo.endCursor}')"
  unresolved=$((unresolved + $(jq .unresolved <<<"$page")))
  [ "$(jq -r .hasNextPage <<<"$page")" = true ] || break
  cursor="$(jq -r .endCursor <<<"$page")"
done

files="$(gh api --paginate "repos/$repo/pulls/$pr/files?per_page=100" \
  --jq '.[] | {path: .filename, additions, deletions, status, previous: .previous_filename, patch}' |
  jq -s '[.[]
    | ({path, additions, deletions, status}
       # Shared Go and test files are reviewed line by line, so keep their patch.
       + (if .path | IN("cmd/server/main.go", "config/config.go", "config/config_test.go", "web/tests/unit/ui.test.ts")
          then {patch} else {} end)),
      # A rename takes its source away, so list the source as removed.
      (select(.previous != null) | {path: .previous, additions: 0, deletions: .deletions, status: "removed", synthetic: true})]')"

# The files endpoint stops at 3000 entries; compare with the PR's own count.
changed_files="$(gh api "repos/$repo/pulls/$pr" --jq .changed_files)"
listed="$(jq '[.[] | select(.synthetic | not)] | length' <<<"$files")"
files_truncated=false
[ "$listed" -ge "$changed_files" ] || files_truncated=true
files="$(jq '[.[] | del(.synthetic)]' <<<"$files")"

merge_base="$(gh api "repos/$repo/compare/$base_path...$head_sha" --jq .merge_base_commit.sha)"
base_integrations="$(gh api "repos/$repo/contents/integrations?ref=$merge_base" --jq '[.[] | select(.type == "dir") | .name]')"

# The files endpoint follows the PR's live head, while checks were read for
# head_sha. If a push landed meanwhile the facts would mix two revisions.
after="$(gh pr view "$pr" --repo "$repo" --json headRefOid,baseRefName)"
if [ "$(jq -r .headRefOid <<<"$after")" != "$head_sha" ] || [ "$(jq -r .baseRefName <<<"$after")" != "$base_ref" ]; then
  echo "PR #$pr changed while its facts were collected; evaluate it again on the next run" >&2
  exit 1
fi

# Large values go through files, not argv: a 3000-file list can exceed the
# per-argument limit on Linux.
printf '%s' "$view" >"$facts_dir/view"
printf '%s' "$files" >"$facts_dir/files"
printf '%s' "$check_runs" >"$facts_dir/check_runs"
printf '%s' "$statuses" >"$facts_dir/statuses"

jq -n \
  --slurpfile view "$facts_dir/view" \
  --slurpfile files "$facts_dir/files" \
  --slurpfile check_runs "$facts_dir/check_runs" \
  --slurpfile statuses "$facts_dir/statuses" \
  --argjson required "$required" \
  --argjson unresolved "$unresolved" \
  --argjson base_integrations "$base_integrations" \
  --argjson files_truncated "$files_truncated" \
  '$view[0] as $view
  | {
    pr: $view.number,
    state: $view.state,
    draft: $view.isDraft,
    cross_repository: $view.isCrossRepository,
    head_ref: $view.headRefName,
    head_sha: $view.headRefOid,
    base_ref: $view.baseRefName,
    labels: [$view.labels[].name],
    required_checks: $required,
    check_runs: $check_runs[0],
    statuses: $statuses[0],
    unresolved_threads: $unresolved,
    review_decision: ($view.reviewDecision // ""),
    mergeable: $view.mergeable,
    merge_state: $view.mergeStateStatus,
    base_integrations: $base_integrations,
    files: $files[0],
    files_truncated: $files_truncated
  }'
