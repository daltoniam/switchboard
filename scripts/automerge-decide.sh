#!/usr/bin/env bash
# Decides whether a bot integration PR may be merged without a person.
#
# Reads the PR facts JSON produced by scripts/automerge-facts.sh on stdin and
# prints {"decision": "merge"|"hold", "integration": <name>, "reasons": [...]}.
# Every condition must hold on the PR's current head; any reason means hold.
set -euo pipefail

jq '
  # Shared docs and generated CSS an integration lists itself in. They may
  # only gain lines. CI regenerates the CSS and fails on any difference.
  def additive_shared: ["README.md", "docs/adapter-reference.md", "web/static/style.css"];
  # Shared Go files an integration registers in. Every added line must be one
  # of the exact registration shapes for this integration (see registration).
  def registration_files: ["cmd/server/main.go", "config/config.go"];

  # Runs of consecutive added lines in a unified diff, and whether it removes any.
  def added_runs:
    reduce (split("\n")[] | select(startswith("@@") | not)) as $line
      ({runs: [], current: []};
       if ($line | startswith("+")) then .current += [$line[1:]]
       elif (.current | length) > 0 then .runs += [.current] | .current = []
       else . end)
    | .runs + (if (.current | length) > 0 then [.current] else [] end);
  def removes_lines: split("\n") | any(startswith("-"));

  # A run of added lines is acceptable registration for $name in $path when:
  #   main.go: one import line, or one constructor call in the registry list
  #   config.go: one "<name>": { ... }, block that maps credentials to
  #     <NAME>_* environment variables or lists disabled-by-default credentials
  def registration($path; $name):
    ($name | ascii_upcase) as $env
    | if $path == "cmd/server/main.go" then
        length == 1 and (.[0] | test("^\t\"github\\.com/daltoniam/switchboard/integrations/" + $name + "\"$")
                               or test("^\t\t" + $name + "\\.New\\(\\),$"))
      else
        length >= 3
        and (.[0] | test("^\t+\"" + $name + "\": \\{$"))
        and (.[-1] | test("^\t+\\},$"))
        and (.[1:-1] | all(
              test("^\t+\"[a-z0-9_]+\":\\s+\"" + $env + "_[A-Z0-9_]+\",$")
              or test("^\t+Enabled:\\s+false,$")
              or test("^\t+Credentials:\\s+mcp\\.Credentials\\{(\"[a-z0-9_]+\": \"\"(, )?)*\\},$")))
      end;
  # Shared test files whose expected counts change when an integration is
  # added. Their patches are reviewed by shared_test_edit.
  def editable_shared_tests: ["config/config_test.go", "web/tests/unit/ui.test.ts"];

  # Change blocks of a unified diff: each consecutive run of -/+ lines,
  # split into what it removes and what it adds.
  def change_blocks:
    reduce ((split("\n") | map(select(startswith("@@") | not))) + [" "])[] as $line
      ({blocks: [], removed: [], added: []};
       if ($line | startswith("-")) then .removed += [$line[1:]]
       elif ($line | startswith("+")) then .added += [$line[1:]]
       elif (.removed + .added | length) > 0
         then .blocks += [{removed, added}] | .removed = [] | .added = []
       else . end)
    | .blocks;

  # Count assertions whose expected number grows by one per integration.
  def count_assertion:
    test("^\\s*assert\\.Len\\(t, [A-Za-z0-9_.]+, [0-9]+\\)$")
    or test("^\\s*assert\\.equal\\(rules\\.length, [0-9]+\\);$");

  # True when $new is $old with exactly one change: the number in a count
  # assertion going up by one, or "<name>" added to a quoted string list.
  def bumped($old; $name):
    . as $new
    | if ($old | count_assertion) and ($new | count_assertion) then
        ($old | capture("^(?<head>.*[^0-9])(?<n>[0-9]+)(?<tail>[^0-9]*)$")) as $o
        | ($new | capture("^(?<head>.*[^0-9])(?<n>[0-9]+)(?<tail>[^0-9]*)$")) as $n
        | $o.head == $n.head and $o.tail == $n.tail and (($n.n | tonumber) - ($o.n | tonumber)) == 1
      else
        # One "<name>" element added inside a []string{...} literal, nowhere else.
        ($old | test("\\[\\]string\\{[^}]*\\}"))
        and ($old | contains("\"" + $name + "\"") | not)
        and ([$new | scan("\"" + $name + "\"")] | length) == 1
        and ($new | test("\\[\\]string\\{[^}]*\"" + $name + "\"[^}]*\\}"))
        and (($new | sub("\"" + $name + "\", "; "")) == $old
             or ($new | sub(", \"" + $name + "\""; "")) == $old)
      end;

  # Added assertion lines a new integration may contribute to shared tests.
  def listing_line($name):
    ($name | ascii_upcase) as $env
    | test("^\t+\"" + $name + "\":\\s+\\{(\"[a-z0-9_]+\"(, )?)*\\},$")
      or test("^\tassert\\.Equal\\(t, \"" + $env + "_[A-Z0-9_]+\", m\\[\"" + $name + "\"\\]\\[\"[a-z0-9_]+\"\\]\\)$")
      or test("^  assert\\.ok\\(rules\\.some\\(\\(\\[, name\\]\\) => name === \u0027" + $name + "\u0027\\)\\);$");

  # Each removed line must come back bumped (in order); extra added lines
  # must be listing lines for this integration.
  def shared_test_edit($name):
    change_blocks | all(
      (.removed | length) <= (.added | length)
      and (. as $b | [range($b.removed | length)] | all(. as $i | $b.added[$i] | bumped($b.removed[$i]; $name)))
      and (.added[(.removed | length):] | all(listing_line($name))));

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
      # Checks outside the required list still must not be failing or running,
      # whether they report as check runs or as commit statuses.
      (([$f.check_runs[] | {name, result: .conclusion}]
        + [$f.statuses[] | {name: .context, result: .state}])[]
        | select((.name as $n | $f.required_checks | index($n)) == null)
        | if (.result | pending) then "waiting for \(.name)"
          elif (.result | passed) then empty
          else "\(.name) is \(.result)" end),

      (if $f.files_truncated then "file list is incomplete (GitHub lists at most 3000 files)" else empty end),
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
        (if ($f.files | any(.path == "web/live_" + $name + "_test.go" and .status != "removed") | not)
          then "no live acceptance spec web/live_\($name)_test.go" else empty end),
        ($f.files[]
          | if (.path | owned($name)) then empty
            # A rename lists its source as removed; only owned files may go.
            elif .status == "removed" and ((.path as $p | additive_shared + registration_files + editable_shared_tests | index($p)) != null)
              then "\(.path) is removed or renamed; shared files may only gain lines"
            elif (.path as $p | registration_files | index($p)) != null then
              .path as $p
              | if (.patch // "") == "" then "\($p) has no patch to review"
                elif .deletions > 0 or (.patch | removes_lines) then "\($p) removes lines; shared files may only gain lines"
                elif (.patch | added_runs | all(registration($p; $name))) then empty
                else "\($p) adds a line that is not \($name) registration" end
            elif (.path as $p | additive_shared | index($p)) != null then
              (if .deletions > 0 then "\(.path) removes lines; shared files may only gain lines" else empty end)
            elif (.path as $p | editable_shared_tests | index($p)) != null then
              .path as $p
              | if (.patch // "") == "" then "\($p) has no patch to review"
                elif (.patch | shared_test_edit($name)) then empty
                else "\($p) changes more than counts and \($name) entries" end
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
