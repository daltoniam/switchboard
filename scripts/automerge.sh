#!/usr/bin/env bash
# Evaluates open bot integration PRs and merges the ones that pass every
# gate in scripts/automerge-decide.sh.
#
# AUTOMERGE_MODE:
#   dry-run   comment "would merge" once per head, never merge (default)
#   merge     add release:skip, then squash-merge at the evaluated head
#   disabled  do nothing
# MERGE_TOKEN, if set, is used only for the merge call so the merge commit
# starts push workflows; GH_TOKEN is used for everything else.
#
# Usage: scripts/automerge.sh [pr-number...]   (default: open overload-automation PRs)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode=${AUTOMERGE_MODE:-dry-run}
facts_cmd=${AUTOMERGE_FACTS:-"$ROOT/scripts/automerge-facts.sh"}
decide_cmd=${AUTOMERGE_DECIDE:-"$ROOT/scripts/automerge-decide.sh"}
summary=${GITHUB_STEP_SUMMARY:-/dev/null}

case "$mode" in
  dry-run | merge) ;;
  disabled) echo "auto-merge is disabled"; exit 0 ;;
  *) echo "unknown AUTOMERGE_MODE $mode (want dry-run, merge or disabled)" >&2; exit 2 ;;
esac

repo=${GITHUB_REPOSITORY:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}

if [ "$#" -gt 0 ]; then
  prs="$*"
else
  prs="$(gh pr list --repo "$repo" --state open --label overload-automation --limit 1000 --json number --jq '.[].number')"
fi

echo "## Auto-merge ($mode)" >>"$summary"
status=0
for pr in $prs; do
  if ! facts="$(bash "$facts_cmd" "$pr")"; then
    echo "#$pr: could not read PR facts" | tee -a "$summary" >&2
    status=1
    continue
  fi
  result="$(bash "$decide_cmd" <<<"$facts")"
  decision="$(jq -r .decision <<<"$result")"
  sha="$(jq -r .head_sha <<<"$facts")"
  integration="$(jq -r '.integration // "unknown"' <<<"$result")"

  if [ "$decision" != merge ]; then
    {
      echo "#$pr ($integration) held at ${sha:0:7}:"
      jq -r '.reasons[] | "- " + .' <<<"$result"
    } | tee -a "$summary"
    continue
  fi

  marker="<!-- automerge:$sha -->"
  if [ "$mode" = dry-run ]; then
    # Read every comment before searching, so a lookup failure is never
    # mistaken for "not noted yet" and turned into a duplicate comment.
    if ! bodies="$(gh api --paginate "repos/$repo/issues/$pr/comments" --jq '.[].body')"; then
      echo "#$pr: could not read existing comments" | tee -a "$summary" >&2
      status=1
      continue
    fi
    if grep -qF -- "$marker" <<<"$bodies"; then
      echo "#$pr ($integration) would merge at ${sha:0:7}; already noted" | tee -a "$summary"
      continue
    fi
    gh pr comment "$pr" --repo "$repo" --body "$(printf '%s\n\n%s\n\n%s' \
      "Auto-merge (dry run): this PR passes every gate at \`$sha\` and would be merged now, with \`release:skip\` so it ships in the weekly release." \
      "Gates: all required checks and the overload review pass on this head, no open review threads, a new \`$integration\` integration with a live acceptance spec, and no changes outside what an integration may touch. Add \`automation-hold\` to keep a PR out of auto-merge." \
      "$marker")"
    echo "#$pr ($integration) would merge at ${sha:0:7}" | tee -a "$summary"
    continue
  fi

  gh pr edit "$pr" --repo "$repo" --add-label release:skip
  if GH_TOKEN="${MERGE_TOKEN:-${GH_TOKEN:-}}" gh pr merge "$pr" --repo "$repo" --squash --match-head-commit "$sha"; then
    echo "#$pr ($integration) merged at ${sha:0:7}" | tee -a "$summary"
  else
    echo "#$pr ($integration) merge failed at ${sha:0:7}" | tee -a "$summary" >&2
    status=1
  fi
done
exit "$status"
