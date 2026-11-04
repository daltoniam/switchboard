#!/usr/bin/env bash
# Decides whether a bot integration PR may be merged without a person.
#
# Reads the PR facts JSON produced by scripts/automerge-facts.sh on stdin and
# prints {"decision": "merge"|"hold", "integration": <name>, "reasons": [...]}.
# Every condition must hold on the PR's current head; any reason means hold.
set -euo pipefail

jq '
  # Shared production files an integration registers itself in. They may
  # only gain lines, so a bot PR cannot change existing behavior there.
  def additive_shared: [
    "cmd/server/main.go", "config/config.go", "README.md",
    "docs/adapter-reference.md", "web/static/style.css"
  ];
  # Shared test files whose expected counts change when an integration is added.
  def editable_shared_tests: ["config/config_test.go", "web/tests/unit/ui.test.ts"];

  def owned($name):
    test("^integrations/" + $name + "/")
    or test("^docs/" + $name + "([-.][a-z0-9.-]*)?\\.md$")
    or test("^web/(" + $name + "_ui|ui_" + $name + "|live_" + $name + ")_test\\.go$")
    or test("^config/" + $name + "_test\\.go$")
    or test("^server/" + $name + "_[a-z0-9_]*test\\.go$")
    or test("^web/templates/pages/" + $name + "_setup(\\.templ|_templ\\.go)$");

  def pending: . == null or . == "" or . == "pending" or . == "queued"
    or . == "in_progress" or . == "waiting" or . == "requested" or . == "expected";
  def passed: . == "success" or . == "neutral" or . == "skipped";

  . as $f
  | (($f.head_ref | capture("^feat/(?<name>[a-z0-9]+)-integration$").name) // null) as $name
  | [
      (if $f.state != "OPEN" then "PR is not open (\($f.state))" else empty end),
      (if $f.draft then "PR is a draft" else empty end),
      (if $f.cross_repository then "PR comes from a fork" else empty end),
      (if $f.base_ref != "main" then "base branch is \($f.base_ref), not main" else empty end),
      (if ($f.labels | index("overload-automation")) == null
        then "missing the overload-automation label" else empty end),
      (if ($f.labels | index("automation-hold")) != null
        then "automation-hold label is set" else empty end),
      (if $name == null
        then "branch \($f.head_ref) is not feat/<name>-integration" else empty end),

      ($f.required_checks[] as $check
        | ([$f.check_runs[] | select(.name == $check) | .conclusion]
           + [$f.statuses[] | select(.context == $check) | .state]) as $results
        | if ($results | length) == 0 or ($results | any(pending)) then "waiting for \($check)"
          elif ($results | all(passed)) then empty
          else "\($check) is \($results | map(select(passed | not)) | unique | join(", "))" end),
      ($f.check_runs[]
        | select((.name as $n | $f.required_checks | index($n)) == null)
        | select((.conclusion | pending) or (.conclusion | passed) | not)
        | "\(.name) is \(.conclusion)"),
      ($f.check_runs[]
        | select((.name as $n | $f.required_checks | index($n)) == null)
        | select(.conclusion | pending)
        | "waiting for \(.name)"),

      (if $f.unresolved_threads > 0
        then "\($f.unresolved_threads) unresolved review threads" else empty end),
      (if $f.review_decision == "CHANGES_REQUESTED" then "a reviewer has changes requested" else empty end),
      (if $f.mergeable != "MERGEABLE" then "not mergeable (\($f.mergeable))" else empty end),
      (if ($f.merge_state | IN("CLEAN", "HAS_HOOKS") | not)
        then "merge state is \($f.merge_state)" else empty end),

      (if $name == null then empty else
        (if ($f.base_integrations | index($name)) != null
          then "integration \($name) already exists on main; only new integrations auto-merge" else empty end),
        (if ($f.files | any(.path | startswith("integrations/" + $name + "/")) | not)
          then "no files under integrations/\($name)/" else empty end),
        (if ($f.files | any(.path == "web/live_" + $name + "_test.go") | not)
          then "no live acceptance spec web/live_\($name)_test.go" else empty end),
        ($f.files[]
          | if (.path | owned($name)) then empty
            elif (.path as $p | additive_shared | index($p)) != null then
              (if .deletions > 0 then "\(.path) removes lines; shared files may only gain lines" else empty end)
            elif (.path as $p | editable_shared_tests | index($p)) != null then empty
            else "\(.path) is outside what an integration PR may change" end)
      end)
    ]
  | unique as $reasons
  | {
      decision: (if ($reasons | length) == 0 then "merge" else "hold" end),
      integration: $name,
      reasons: $reasons
    }
'
