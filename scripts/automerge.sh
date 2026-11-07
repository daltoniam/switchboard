#!/usr/bin/env bash
# Evaluates open bot integration PRs and merges the ones that pass every
# gate in scripts/automerge-decide.sh.
#
# AUTOMERGE_MODE:
#   dry-run   comment "would merge" once per head, never merge (default)
#   merge     add release:skip, then squash-merge at the evaluated head
#   disabled  do nothing
# AUTOFIX (on by default, "off" to disable): when a held PR is something the
#   bot can fix (conflict with main, failed check, review findings), start
#   the implementation job in shepherd mode for it. One PR per run, only
#   while no implementation run is queued or running, and at most twice per
#   head and set of findings: a run can be lost with its runner, but a fix
#   the job cannot make must not loop.
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

autofix=${AUTOFIX:-on}

# comments <pr>: every comment body, or failure. Read in full before
# searching, so a lookup failure is never mistaken for "not there yet".
comments() { gh api --paginate "repos/$repo/issues/$1/comments" --jq '.[].body'; }

echo "## Auto-merge ($mode)" >>"$summary"
status=0
fixable=()
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
    next="$(jq -r '.next // "wait"' <<<"$result")"
    case "$next" in
      person) label="needs a person" ;;
      fix) label="needs fixes"; fixable+=("$pr|$sha|$(jq -c '.reasons' <<<"$result")") ;;
      *) label="waiting" ;;
    esac
    {
      echo "#$pr ($integration) $label at ${sha:0:7}:"
      jq -r '.reasons[] | "- " + .' <<<"$result"
    } | tee -a "$summary"
    continue
  fi

  marker="<!-- automerge:$sha -->"
  if [ "$mode" = dry-run ]; then
    if ! bodies="$(comments "$pr")"; then
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

  added_label=false
  if ! jq -e '.labels | index("release:skip")' <<<"$facts" >/dev/null; then
    # The label may not exist yet; create it once and retry. A PR that
    # still cannot be labeled is not merged, since it would release alone.
    if ! gh pr edit "$pr" --repo "$repo" --add-label release:skip; then
      gh label create release:skip --repo "$repo" --color ededed \
        --description "Ship in the weekly release instead of releasing on merge" >/dev/null 2>&1 || true
      if ! gh pr edit "$pr" --repo "$repo" --add-label release:skip; then
        echo "#$pr ($integration) not merged: could not add release:skip" | tee -a "$summary" >&2
        status=1
        continue
      fi
    fi
    added_label=true
  fi
  if GH_TOKEN="${MERGE_TOKEN:-${GH_TOKEN:-}}" gh pr merge "$pr" --repo "$repo" --squash --match-head-commit "$sha"; then
    echo "#$pr ($integration) merged at ${sha:0:7}" | tee -a "$summary"
  else
    # Leave the PR as it was, so a later manual merge still releases.
    if [ "$added_label" = true ]; then
      gh pr edit "$pr" --repo "$repo" --remove-label release:skip || true
    fi
    echo "#$pr ($integration) merge failed at ${sha:0:7}" | tee -a "$summary" >&2
    status=1
  fi
done

if [ "$autofix" = on ] && [ "${#fixable[@]}" -gt 0 ]; then
  # The implementation job runs one pass at a time; a second dispatch would
  # only replace a queued one, so wait until it is idle.
  # Ask for each active status directly; a long run can be older than any
  # fixed window of recent runs.
  busy=0
  for state in in_progress queued waiting pending requested; do
    count="$(gh run list --workflow overload-implement.yml --repo "$repo" --status "$state" --limit 1 --json databaseId --jq length)"
    busy=$((busy + count))
  done
  if [ "$busy" != 0 ]; then
    echo "Auto-fix: the implementation job is busy; fixable PRs wait for the next run" | tee -a "$summary"
  else
    for entry in "${fixable[@]}"; do
      IFS='|' read -r pr sha reasons <<<"$entry"
      key="$(printf '%s' "$reasons" | cksum | cut -d' ' -f1)"
      marker="<!-- autofix:$sha:$key -->"
      if ! bodies="$(comments "$pr")"; then
        echo "#$pr: could not read existing comments" | tee -a "$summary" >&2
        status=1
        continue
      fi
      # Two attempts per head and set of findings: a run can be lost with
      # its runner, but a fix the job cannot make must not loop.
      attempts="$(grep -cF -- "$marker" <<<"$bodies" || true)"
      if [ "$attempts" -ge 2 ]; then
        continue
      fi
      attempt=$((attempts + 1))
      gh workflow run overload-implement.yml --repo "$repo" -f mode=shepherd -f target="$pr"
      gh pr comment "$pr" --repo "$repo" --body "$(printf '%s\n\n%s\n\n%s' \
        "Auto-fix: started the implementation job to fix this PR at \`$sha\` (attempt $attempt of 2)." \
        "$(jq -r '.[] | "- " + .' <<<"$reasons")" \
        "$marker")"
      echo "Auto-fix: sent the implementation job to #$pr at ${sha:0:7}" | tee -a "$summary"
      break
    done
  fi
fi
exit "$status"
