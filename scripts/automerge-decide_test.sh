#!/usr/bin/env bash
# Verifies the auto-merge decision for bot integration PRs.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DECIDE="${ROOT}/scripts/automerge-decide.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*" >&2; }

# A bot PR adding the keycloak integration that is ready to merge.
ready='{
  "pr": 359, "state": "OPEN", "draft": false, "cross_repository": false,
  "head_ref": "feat/keycloak-integration", "base_ref": "main",
  "head_sha": "abc123",
  "labels": ["overload-automation", "integration", "enhancement"],
  "required_checks": ["build", "test", "overload", "live-integration"],
  "check_runs": [
    {"name": "build", "conclusion": "success"},
    {"name": "test", "conclusion": "success"},
    {"name": "live-integration", "conclusion": "success"},
    {"name": "Crush PR Review", "conclusion": "success"}
  ],
  "statuses": [{"context": "overload", "state": "success"}],
  "unresolved_threads": 0,
  "review_decision": "APPROVED",
  "mergeable": "MERGEABLE", "merge_state": "CLEAN",
  "base_integrations": ["github", "postgres"],
  "files": [
    {"path": "integrations/keycloak/keycloak.go", "additions": 300, "deletions": 0},
    {"path": "integrations/keycloak/compact.yaml", "additions": 10, "deletions": 0},
    {"path": "web/live_keycloak_test.go", "additions": 120, "deletions": 0},
    {"path": "web/keycloak_ui_test.go", "additions": 180, "deletions": 0},
    {"path": "docs/keycloak-setup.md", "additions": 50, "deletions": 0},
    {"path": "config/keycloak_test.go", "additions": 28, "deletions": 0},
    {"path": "server/keycloak_test.go", "additions": 105, "deletions": 0},
    {"path": "cmd/server/main.go", "additions": 2, "deletions": 0},
    {"path": "config/config.go", "additions": 10, "deletions": 0},
    {"path": "config/config_test.go", "additions": 6, "deletions": 5},
    {"path": "README.md", "additions": 4, "deletions": 0},
    {"path": "docs/adapter-reference.md", "additions": 4, "deletions": 0},
    {"path": "web/static/style.css", "additions": 1, "deletions": 0},
    {"path": "web/tests/unit/ui.test.ts", "additions": 2, "deletions": 1}
  ]
}'

# check <description> <jq edit applied to $ready> <expected decision> [expected reason substring]
check() {
  local description=$1 edit=$2 want=$3 reason=${4:-} facts output decision reasons
  facts="$(jq "$edit" <<<"$ready")"
  output="$(bash "$DECIDE" <<<"$facts")" || fail "$description: decide exited $?"
  decision="$(jq -r .decision <<<"$output")"
  reasons="$(jq -r '.reasons | join("\n")' <<<"$output")"
  [ "$decision" = "$want" ] || fail "$description: decision $decision, want $want; reasons: $reasons"
  if [ -n "$reason" ] && ! grep -qF -- "$reason" <<<"$reasons"; then
    fail "$description: reasons lack [$reason]; got: $reasons"
  fi
  if [ "$want" = merge ] && [ -n "$reasons" ]; then
    fail "$description: merge decision carries reasons: $reasons"
  fi
  ok "$description"
}

check "ready bot PR merges" '.' merge
check "integration name is reported" '.' merge
[ "$(bash "$DECIDE" <<<"$ready" | jq -r .integration)" = keycloak ] || fail "integration name not reported"

check "closed PR holds" '.state = "MERGED"' hold "PR is not open"
check "draft holds" '.draft = true' hold "PR is a draft"
check "fork holds" '.cross_repository = true' hold "fork"
check "human PR holds" '.labels = ["integration"]' hold "overload-automation"
check "kill switch holds" '.labels += ["automation-hold"]' hold "automation-hold"
check "non-integration branch holds" '.head_ref = "fix/something"' hold "feat/<name>-integration"
check "non-main base holds" '.base_ref = "ui/release-x"' hold "base branch"

check "missing required check waits" '.check_runs |= map(select(.name != "live-integration"))' hold "waiting for live-integration"
check "running required check waits" '.check_runs |= map(if .name == "test" then .conclusion = "in_progress" else . end)' hold "waiting for test"
check "failed required check holds" '.check_runs |= map(if .name == "build" then .conclusion = "failure" else . end)' hold "build is failure"
check "pending overload waits" '.statuses = [{"context": "overload", "state": "pending"}]' hold "waiting for overload"
check "overload changes requested holds" '.statuses = [{"context": "overload", "state": "failure"}]' hold "overload is failure"
check "failed optional check holds" '.check_runs |= map(if .name == "Crush PR Review" then .conclusion = "failure" else . end)' hold "Crush PR Review is failure"
check "skipped optional check is fine" '.check_runs += [{"name": "docs", "conclusion": "skipped"}]' merge

check "open review thread holds" '.unresolved_threads = 2' hold "2 unresolved review threads"
check "requested changes hold" '.review_decision = "CHANGES_REQUESTED"' hold "changes requested"
check "conflict holds" '.mergeable = "CONFLICTING"' hold "not mergeable"
check "blocked merge state holds" '.merge_state = "BLOCKED"' hold "merge state is BLOCKED"

check "existing integration holds" '.base_integrations += ["keycloak"]' hold "keycloak already exists"
check "branch and directory mismatch holds" '.head_ref = "feat/sso-integration"' hold "no files under integrations/sso/"
check "missing live spec holds" '.files |= map(select(.path != "web/live_keycloak_test.go"))' hold "web/live_keycloak_test.go"
check "workflow change holds" '.files += [{"path": ".github/workflows/ci.yml", "additions": 1, "deletions": 0}]' hold ".github/workflows/ci.yml"
check "server change holds" '.files += [{"path": "server/server.go", "additions": 1, "deletions": 0}]' hold "server/server.go"
check "dependency change holds" '.files += [{"path": "go.mod", "additions": 3, "deletions": 0}]' hold "go.mod"
check "other integration holds" '.files += [{"path": "integrations/github/github.go", "additions": 1, "deletions": 0}]' hold "integrations/github/github.go"
check "other integration live spec holds" '.files += [{"path": "web/live_postgres_test.go", "additions": 1, "deletions": 0}]' hold "web/live_postgres_test.go"
check "removing lines from shared code holds" '.files |= map(if .path == "cmd/server/main.go" then .deletions = 1 else . end)' hold "cmd/server/main.go removes lines"
check "editing shared tests is fine" '.files |= map(if .path == "config/config_test.go" then .deletions = 20 else . end)' merge
check "lookalike integration name holds" '.files += [{"path": "integrations/keycloak2/x.go", "additions": 1, "deletions": 0}]' hold "integrations/keycloak2/x.go"

echo "PASS: automerge decide" >&2
