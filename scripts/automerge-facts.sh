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

required="$(gh api "repos/$repo/rules/branches/$base_ref" \
  --jq '[.[] | select(.type == "required_status_checks") | .parameters.required_status_checks[].context] | unique')"

# Latest run per check name; a pending run has no conclusion yet, so report
# its status instead.
check_runs="$(gh api --paginate "repos/$repo/commits/$head_sha/check-runs?per_page=100" \
  --jq '.check_runs[] | {name, conclusion: (.conclusion // .status), started_at}' |
  jq -s 'group_by(.name) | map(sort_by(.started_at) | last | {name, conclusion})')"

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
       # Registration files are reviewed line by line, so keep their patch.
       + (if .path == "cmd/server/main.go" or .path == "config/config.go" then {patch} else {} end)),
      # A rename takes its source away, so list the source as removed.
      (select(.previous != null) | {path: .previous, additions: 0, deletions: .deletions, status: "removed"})]')"

merge_base="$(gh api "repos/$repo/compare/$base_ref...$head_sha" --jq .merge_base_commit.sha)"
base_integrations="$(gh api "repos/$repo/contents/integrations?ref=$merge_base" --jq '[.[] | select(.type == "dir") | .name]')"

jq -n \
  --argjson view "$view" \
  --argjson required "$required" \
  --argjson check_runs "$check_runs" \
  --argjson statuses "$statuses" \
  --argjson unresolved "$unresolved" \
  --argjson files "$files" \
  --argjson base_integrations "$base_integrations" \
  '{
    pr: $view.number,
    state: $view.state,
    draft: $view.isDraft,
    cross_repository: $view.isCrossRepository,
    head_ref: $view.headRefName,
    head_sha: $view.headRefOid,
    base_ref: $view.baseRefName,
    labels: [$view.labels[].name],
    required_checks: $required,
    check_runs: $check_runs,
    statuses: $statuses,
    unresolved_threads: $unresolved,
    review_decision: ($view.reviewDecision // ""),
    mergeable: $view.mergeable,
    merge_state: $view.mergeStateStatus,
    base_integrations: $base_integrations,
    files: $files
  }'
