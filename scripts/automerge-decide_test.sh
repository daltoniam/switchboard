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
  "required_checks": [
    {"context": "build", "app_id": 15368}, {"context": "test", "app_id": 15368},
    {"context": "overload", "app_id": null}, {"context": "live-integration", "app_id": 15368}
  ],
  "check_runs": [
    {"name": "build", "app_id": 15368, "conclusion": "success"},
    {"name": "test", "app_id": 15368, "conclusion": "success"},
    {"name": "live-integration", "app_id": 15368, "conclusion": "success"},
    {"name": "Crush PR Review", "app_id": 15368, "conclusion": "success"}
  ],
  "statuses": [{"context": "overload", "state": "success"}],
  "unresolved_threads": 0,
  "review_decision": "APPROVED",
  "mergeable": "MERGEABLE", "merge_state": "CLEAN", "files_truncated": false,
  "base_integrations": ["github", "postgres"],
  "files": [
    {"path": "integrations/keycloak/keycloak.go", "additions": 300, "deletions": 0},
    {"path": "integrations/keycloak/compact.yaml", "additions": 10, "deletions": 0},
    {"path": "web/live_keycloak_test.go", "additions": 120, "deletions": 0},
    {"path": "web/keycloak_ui_test.go", "additions": 180, "deletions": 0},
    {"path": "docs/keycloak-setup.md", "additions": 50, "deletions": 0},
    {"path": "config/keycloak_test.go", "additions": 28, "deletions": 0},
    {"path": "server/keycloak_test.go", "additions": 105, "deletions": 0},
    {"path": "cmd/server/main.go", "additions": 2, "deletions": 0,
     "patch": "@@ -57,6 +57,7 @@ import (\n \t\"github.com/daltoniam/switchboard/integrations/jira\"\n+\t\"github.com/daltoniam/switchboard/integrations/keycloak\"\n \t\"github.com/daltoniam/switchboard/integrations/kubernetes\"\n@@ -365,6 +366,7 @@ func runServer(\n \t\tokta.New(),\n+\t\tkeycloak.New(),\n \t\tollama.New(),"},
    {"path": "config/config.go", "additions": 8, "deletions": 0,
     "patch": "@@ -241,6 +241,12 @@ var envMapping = map[string]map[string]string{\n \t},\n+\t\"keycloak\": {\n+\t\t\"base_url\":      \"KEYCLOAK_BASE_URL\",\n+\t\t\"client_secret\": \"KEYCLOAK_CLIENT_SECRET\",\n+\t},\n \t\"pagerduty\": {\n@@ -520,6 +526,10 @@ func defaultConfig() *mcp.Config {\n \t\t\t},\n+\t\t\t\"keycloak\": {\n+\t\t\t\tEnabled:     false,\n+\t\t\t\tCredentials: mcp.Credentials{\"base_url\": \"\", \"client_secret\": \"\"},\n+\t\t\t},\n \t\t\t\"pagerduty\": {"},
    {"path": "config/config_test.go", "additions": 5, "deletions": 2,
     "patch": "@@ -32,7 +32,7 @@ func TestLoad(t *testing.T) {\n-\tassert.Len(t, m.cfg.Integrations, 73)\n+\tassert.Len(t, m.cfg.Integrations, 74)\n@@ -32,8 +32,8 @@\n-\tfor _, name := range []string{\"github\", \"okta\"} {\n+\tfor _, name := range []string{\"github\", \"okta\", \"keycloak\"} {\n@@ -310,6 +310,7 @@ func TestDefaultConfig(t *testing.T) {\n+\t\t\"keycloak\":         {\"base_url\", \"realm\", \"client_id\", \"client_secret\"},\n@@ -704,6 +705,7 @@ func TestEnvMapping_ReturnsMapping(t *testing.T) {\n+\tassert.Equal(t, \"KEYCLOAK_BASE_URL\", m[\"keycloak\"][\"base_url\"])"},
    {"path": "README.md", "additions": 4, "deletions": 0},
    {"path": "docs/adapter-reference.md", "additions": 4, "deletions": 0},
    {"path": "web/static/style.css", "additions": 1, "deletions": 0},
    {"path": "web/tests/unit/ui.test.ts", "additions": 2, "deletions": 1,
     "patch": "@@ -17,7 +17,8 @@ test(\"logos\", () => {\n-  assert.equal(rules.length, 60);\n+  assert.equal(rules.length, 61);\n+  assert.ok(rules.some(([, name]) => name === '\''keycloak'\''));"}
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
# Registration files only accept the exact registration lines for this integration.
patch_main() { printf '.files |= map(if .path == "cmd/server/main.go" then .patch = %s else . end)' "$(jq -Rs . <<<"$1")"; }
patch_config() { printf '.files |= map(if .path == "config/config.go" then .patch = %s else . end)' "$(jq -Rs . <<<"$1")"; }
check "main.go statement added inside a function holds" "$(patch_main $'@@ -1,3 +1,4 @@\n \t\tokta.New(),\n+\t\tos.Setenv("X", "y")\n \t\tollama.New(),')" hold "cmd/server/main.go adds a line that is not keycloak registration"
check "main.go comment opener holds" "$(patch_main $'@@ -1,3 +1,4 @@\n \t\tokta.New(),\n+/*\n \t\tollama.New(),')" hold "cmd/server/main.go adds a line that is not keycloak registration"
check "registering another integration holds" "$(patch_main $'@@ -1,3 +1,4 @@\n \t\tokta.New(),\n+\t\tgithub.New(),\n \t\tollama.New(),')" hold "cmd/server/main.go adds a line that is not keycloak registration"
check "missing patch holds" '.files |= map(if .path == "config/config.go" then del(.patch) else . end)' hold "config/config.go has no patch to review"
check "env mapping to another product's variable holds" "$(patch_config $'@@ -1,2 +1,5 @@\n \t},\n+\t"keycloak": {\n+\t\t"client_secret": "GITHUB_TOKEN",\n+\t},\n \t"pagerduty": {')" hold "config/config.go adds a line that is not keycloak registration"
check "config block for another name holds" "$(patch_config $'@@ -1,2 +1,5 @@\n \t},\n+\t"github": {\n+\t\t"token": "KEYCLOAK_TOKEN",\n+\t},\n \t"pagerduty": {')" hold "config/config.go adds a line that is not keycloak registration"
check "config line outside a block holds" "$(patch_config $'@@ -1,2 +1,3 @@\n \t\t"api_token": "OKTA_API_TOKEN",\n+\t\t"base_url": "KEYCLOAK_BASE_URL",\n \t},')" hold "config/config.go adds a line that is not keycloak registration"
check "env names may separate the integration name with underscores" "$(patch_config $'@@ -1,2 +1,5 @@\n \t},\n+\t"keycloak": {\n+\t\t"base_url": "KEY_CLOAK_BASE_URL",\n+\t},\n \t"pagerduty": {\n@@ -9,2 +12,5 @@\n \t\t\t},\n+\t\t\t"keycloak": {\n+\t\t\t\tEnabled:     false,\n+\t\t\t},\n \t\t\t"pagerduty": {')" merge
check "env name that only starts with the integration name holds" "$(patch_config $'@@ -1,2 +1,5 @@\n \t},\n+\t"keycloak": {\n+\t\t"base_url": "KEYCLOAKX_BASE_URL",\n+\t},\n \t"pagerduty": {')" hold "config/config.go adds a line that is not keycloak registration"
check "config default enabled holds" "$(patch_config $'@@ -1,2 +1,5 @@\n \t\t\t},\n+\t\t\t"keycloak": {\n+\t\t\t\tEnabled:     true,\n+\t\t\t},\n \t\t\t"pagerduty": {')" hold "config/config.go adds a line that is not keycloak registration"

check "failing optional status holds" '.statuses += [{"context": "codecov", "state": "failure"}]' hold "codecov is failure"
check "pending optional status waits" '.statuses += [{"context": "codecov", "state": "pending"}]' hold "waiting for codecov"

check "renaming a shared file into the integration holds" '.files += [{"path": "integrations/keycloak/notes.md", "additions": 0, "deletions": 0}, {"path": "docs/architecture.md", "additions": 0, "deletions": 0, "status": "removed"}]' hold "docs/architecture.md is outside what an integration PR may change"
check "renaming a listing file into the integration holds" '.files += [{"path": "README.md", "additions": 0, "deletions": 0, "status": "removed"}]' hold "README.md is removed"
check "removing an owned file is fine" '.files += [{"path": "integrations/keycloak/old.go", "additions": 0, "deletions": 4, "status": "removed"}]' merge

check "removed live spec holds" '.files |= map(if .path == "web/live_keycloak_test.go" then .status = "removed" else . end)' hold "web/live_keycloak_test.go"
check "incomplete file list holds" '.files_truncated = true' hold "file list is incomplete"

# Shared tests only accept count bumps, this name added to a list, and the
# exact assertion shapes that list this integration.
patch_cfgt() { printf '.files |= map(if .path == "config/config_test.go" then .patch = %s else . end)' "$(jq -Rs . <<<"$1")"; }
check "deleting a shared test line holds" "$(patch_cfgt $'@@ -1,3 +1,2 @@\n-\tassert.Len(t, m, 62)')" hold "config/config_test.go changes more than counts and keycloak entries"
check "rewriting a shared assertion holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tassert.Len(t, m, 62)\n+\tassert.True(t, true)')" hold "config/config_test.go changes more than counts and keycloak entries"
check "a count jump of more than one holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tassert.Len(t, m, 62)\n+\tassert.Len(t, m, 64)')" hold "config/config_test.go changes more than counts and keycloak entries"
check "a skip mentioning the integration holds" "$(patch_cfgt $'@@ -1,1 +1,2 @@\n+\tt.Skip() // "keycloak"')" hold "config/config_test.go changes more than counts and keycloak entries"
check "another integration's entry holds" "$(patch_cfgt $'@@ -1,1 +1,2 @@\n+\t\t"github":         {"token"},')" hold "config/config_test.go changes more than counts and keycloak entries"
check "bumping a loop start in a shared test holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tfor i := 0; i < len(configs); i++ {\n+\tfor i := 1; i < len(configs); i++ {')" hold "config/config_test.go changes more than counts and keycloak entries"
check "bumping a non-count argument holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tassert.Equal(t, 3, retries)\n+\tassert.Equal(t, 4, retries)')" hold "config/config_test.go changes more than counts and keycloak entries"
check "adding the name and bumping a number on one line holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tcheck(t, 2, []string{"github"})\n+\tcheck(t, 3, []string{"github", "keycloak"})')" hold "config/config_test.go changes more than counts and keycloak entries"
check "inserting the name as assertion arguments holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tassert.Equal(t, want, got)\n+\tassert.Equal(t, "keycloak", "keycloak", want, got)')" hold "config/config_test.go changes more than counts and keycloak entries"
check "appending the name as a message argument holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tassert.Equal(t, want, got)\n+\tassert.Equal(t, want, got, "keycloak")')" hold "config/config_test.go changes more than counts and keycloak entries"
check "adding the name to a string list twice holds" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tnames := []string{"github"}\n+\tnames := []string{"keycloak", "github", "keycloak"}')" hold "config/config_test.go changes more than counts and keycloak entries"
check "adding the name at the start of a string list is fine" "$(patch_cfgt $'@@ -1,1 +1,1 @@\n-\tfor _, name := range []string{"github", "okta"} {\n+\tfor _, name := range []string{"keycloak", "github", "okta"} {')" merge
check "required check from another app does not count" '.check_runs |= map(if .name == "build" then .app_id = 999 else . end)' hold "waiting for build"
check "another app cannot hide a failing required check" '.check_runs |= map(if .name == "build" then .conclusion = "failure" else . end) | .check_runs += [{"name": "build", "app_id": 999, "conclusion": "success"}]' hold "build is failure"
check "unrestricted required check accepts a status" '.required_checks += [{"context": "codecov", "app_id": null}] | .statuses += [{"context": "codecov", "state": "success"}]' merge

check "server production file named like a test holds" '.files += [{"path": "server/keycloak_contest.go", "additions": 5, "deletions": 0}]' hold "server/keycloak_contest.go is outside"
check "server test file with a suffix is fine" '.files += [{"path": "server/keycloak_search_test.go", "additions": 5, "deletions": 0}]' merge
check "same-named check from another app failing holds" '.check_runs += [{"name": "build", "app_id": 999, "conclusion": "failure"}]' hold "build is failure"
check "same-named check from another app running waits" '.check_runs += [{"name": "build", "app_id": 999, "conclusion": "in_progress"}]' hold "waiting for build"
check "status named like an app-bound requirement failing holds" '.statuses += [{"context": "build", "state": "failure"}]' hold "build is failure"

# An integration named "adapter" must not own the shared docs/adapter-reference.md.
as_adapter='walk(if type == "string" then gsub("keycloak"; "adapter") | gsub("KEYCLOAK"; "ADAPTER") else . end)'
check "shared doc matching the integration name stays additive-only" "$as_adapter | .files |= map(if .path == \"docs/adapter-reference.md\" then .deletions = 3 else . end)" hold "docs/adapter-reference.md removes lines"
check "shared doc matching the integration name can still grow" "$as_adapter" merge

check "missing server registration holds" '.files |= map(select(.path != "cmd/server/main.go"))' hold "cmd/server/main.go does not register keycloak"
check "import without constructor holds" "$(patch_main $'@@ -1,2 +1,3 @@\n \t"github.com/daltoniam/switchboard/integrations/jira"\n+\t"github.com/daltoniam/switchboard/integrations/keycloak"\n \t"github.com/daltoniam/switchboard/integrations/kubernetes"')" hold "cmd/server/main.go does not register keycloak"
check "missing default config holds" '.files |= map(select(.path != "config/config.go"))' hold "config/config.go has no keycloak defaults"
check "missing shared test patch holds" '.files |= map(if .path == "web/tests/unit/ui.test.ts" then del(.patch) else . end)' hold "web/tests/unit/ui.test.ts has no patch to review"
check "lookalike integration name holds" '.files += [{"path": "integrations/keycloak2/x.go", "additions": 1, "deletions": 0}]' hold "integrations/keycloak2/x.go"

# next tells the runner what to do with a held PR: wait for running checks,
# send the bot to fix it, or leave it for a person.
check_next() {
  local description=$1 edit=$2 want=$3 got
  got="$(jq "$edit" <<<"$ready" | bash "$DECIDE" | jq -r .next)"
  [ "$got" = "$want" ] || fail "$description: next $got, want $want"
  ok "$description"
}
check_next "ready PR is next to merge" '.' merge
check_next "running check waits" '.check_runs |= map(if .name == "test" then .conclusion = "in_progress" else . end)' wait
check_next "pending overload waits" '.statuses = [{"context": "overload", "state": "pending"}]' wait
check_next "unknown mergeability waits" '.mergeable = "UNKNOWN" | .merge_state = "UNKNOWN"' wait
check_next "failed check goes to the bot" '.check_runs |= map(if .name == "build" then .conclusion = "failure" else . end)' fix
check_next "overload changes requested goes to the bot" '.statuses = [{"context": "overload", "state": "failure"}]' fix
check_next "open review threads go to the bot" '.unresolved_threads = 1' fix
check_next "missing live spec goes to the bot" '.files |= map(select(.path != "web/live_keycloak_test.go"))' fix
check_next "conflict goes to the bot even while checks are missing" '.mergeable = "CONFLICTING" | .merge_state = "DIRTY" | .check_runs |= map(select(.name != "live-integration"))' fix
check_next "failure and a running check still waits" '.check_runs |= map(if .name == "build" then .conclusion = "failure" elif .name == "test" then .conclusion = "in_progress" else . end)' wait
check_next "a failed check named like a merge state still goes to the bot" '.check_runs += [{"name": "UNKNOWN", "app_id": 1, "conclusion": "failure"}] | .merge_state = "BLOCKED"' fix
check_next "a failed check named like a conflict is not a conflict" '.check_runs += [{"name": "CONFLICTING", "app_id": 1, "conclusion": "failure"}] | .check_runs |= map(if .name == "test" then .conclusion = "in_progress" else . end)' wait
check_next "a failed check named like a person reason still goes to the bot" '.check_runs += [{"name": "PR is a draft", "app_id": 1, "conclusion": "failure"}]' fix
check_next "a failed check named like a waiting reason still goes to the bot" '.check_runs += [{"name": "waiting for deployment", "app_id": 1, "conclusion": "failure"}]' fix
check_next "a check named like a scope reason cannot hide the scope reason" '.check_runs += [{"name": "ci is outside what an integration PR may change", "app_id": 1, "conclusion": "in_progress"}] | .files += [{"path": "waiting for ci", "additions": 1, "deletions": 0}] | .mergeable = "CONFLICTING" | .merge_state = "DIRTY"' person
check_next "dependency change needs a person" '.files += [{"path": "go.mod", "additions": 3, "deletions": 0}]' person
check_next "dependency change with a conflict still needs a person" '.files += [{"path": "go.mod", "additions": 3, "deletions": 0}] | .mergeable = "CONFLICTING" | .merge_state = "DIRTY"' person
check_next "existing integration needs a person" '.base_integrations += ["keycloak"]' person
check_next "kill switch needs a person" '.labels += ["automation-hold"]' person
check_next "draft needs a person" '.draft = true' person
check_next "fork needs a person" '.cross_repository = true' person

echo "PASS: automerge decide" >&2
