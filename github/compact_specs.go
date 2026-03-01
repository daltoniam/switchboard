package github

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// rawFieldCompactionSpecs maps tool names to dot-notation field compaction specs.
// Only list/search tools get specs — get tools return full detail,
// mutation tools return confirmation responses.
//
// These specs must let the LLM answer common GitHub questions from a single list call:
//   - "Which PRs need review?" → needs requested_reviewers, labels, assignees
//   - "What issues are in this sprint?" → needs milestone, state, assignees
//   - "Find the CI run that broke the build" → needs status, conclusion, actor, head_sha
//   - "Which step failed in this job?" → needs steps with name + conclusion
//   - "Show recent activity on this repo" → needs event type + payload context
//   - "What notifications do I have and where?" → needs repo name + subject
//   - "Search for issues about X across repos" → needs repo context on each result
var rawFieldCompactionSpecs = map[string][]string{
	// ── Repositories ──────────────────────────────────────────────────
	"github_list_user_repos": {
		"full_name", "description", "language", "stargazers_count",
		"html_url", "private", "archived", "default_branch", "updated_at",
	},
	"github_list_org_repos": {
		"full_name", "description", "language", "stargazers_count",
		"html_url", "private", "archived", "default_branch", "updated_at",
	},
	"github_list_branches": {"name", "commit.sha", "protected"},
	"github_list_tags":     {"name", "commit.sha"},
	"github_list_contributors": {
		"login", "contributions", "html_url", "type",
	},
	"github_list_forks": {
		"full_name", "owner.login", "html_url", "created_at", "default_branch",
	},
	"github_list_collaborators": {"login", "html_url", "type", "permissions"},
	"github_list_repo_teams":    {"name", "slug", "permission"},
	"github_list_autolinks":     {"id", "key_prefix", "url_template", "is_alphanumeric"},

	// ── Issues ────────────────────────────────────────────────────────
	"github_list_issues": {
		"number", "title", "state", "html_url", "created_at", "updated_at",
		"comments", "user.login", "labels[].name", "assignees[].login",
	},
	"github_list_issue_comments": {
		"id", "body", "user.login", "created_at", "html_url",
	},
	"github_list_milestones": {
		"number", "title", "state", "open_issues", "closed_issues",
		"due_on", "html_url",
	},
	"github_list_issue_events": {
		"id", "event", "actor.login", "created_at", "label.name",
		"assignee.login", "milestone.title",
	},
	"github_list_issue_timeline": {
		"id", "event", "actor.login", "created_at",
	},
	"github_list_assignees": {"login", "html_url", "type"},

	// ── Pull Requests ─────────────────────────────────────────────────
	"github_list_prs": {
		"number", "title", "state", "html_url", "created_at", "updated_at",
		"draft", "user.login", "head.ref", "base.ref",
		"additions", "deletions", "changed_files",
	},
	"github_list_pr_commits": {
		"sha", "commit.message", "commit.author.name", "commit.author.date", "html_url",
	},
	"github_list_pr_files": {
		"filename", "status", "additions", "deletions", "changes",
	},
	"github_list_pr_reviews": {
		"id", "state", "user.login", "submitted_at", "html_url",
	},
	"github_list_pr_comments": {
		"id", "body", "user.login", "created_at", "path", "line", "html_url",
	},

	// ── Git (low-level) ───────────────────────────────────────────────
	"github_list_commits": {
		"sha", "commit.message", "commit.author.name", "commit.author.date", "html_url",
	},

	// ── Users ─────────────────────────────────────────────────────────
	"github_list_user_followers": {"login", "html_url", "type"},
	"github_list_user_following": {"login", "html_url", "type"},
	"github_list_user_keys":      {"id", "key"},

	// ── Organizations ─────────────────────────────────────────────────
	"github_list_org_members":  {"login", "html_url", "type"},
	"github_list_org_teams":    {"name", "slug", "permission", "description"},
	"github_list_team_members": {"login", "html_url", "type"},
	"github_list_team_repos": {
		"full_name", "description", "language", "html_url", "private",
	},

	// ── Actions (CI/CD) ───────────────────────────────────────────────
	"github_list_workflows": {
		"id", "name", "state", "path", "html_url",
	},
	"github_list_workflow_runs": {
		"id", "name", "status", "conclusion", "head_branch",
		"created_at", "html_url", "event", "run_number",
	},
	"github_list_workflow_jobs": {
		"id", "name", "status", "conclusion", "started_at",
		"completed_at", "html_url",
	},
	"github_list_repo_secrets":        {"name", "created_at", "updated_at"},
	"github_list_artifacts":           {"id", "name", "size_in_bytes", "created_at", "expired"},
	"github_list_environment_secrets": {"name", "created_at", "updated_at"},
	"github_list_org_secrets":         {"name", "created_at", "updated_at"},

	// ── Checks ────────────────────────────────────────────────────────
	"github_list_check_runs": {
		"id", "name", "status", "conclusion", "started_at",
		"completed_at", "html_url",
	},
	"github_list_check_suites": {
		"id", "status", "conclusion", "head_branch", "created_at",
	},

	// ── Releases ──────────────────────────────────────────────────────
	"github_list_releases": {
		"id", "tag_name", "name", "draft", "prerelease",
		"created_at", "html_url",
	},
	"github_list_release_assets": {
		"id", "name", "size", "download_count", "browser_download_url",
		"created_at",
	},

	// ── Gists ─────────────────────────────────────────────────────────
	"github_list_gists": {
		"id", "description", "html_url", "created_at", "updated_at", "public",
	},

	// ── Activity ──────────────────────────────────────────────────────
	"github_list_stargazers":    {"login", "html_url", "type"},
	"github_list_watchers":      {"login", "html_url", "type"},
	"github_list_notifications": {"id", "reason", "subject.title", "subject.type", "updated_at", "unread"},
	"github_list_repo_events": {
		"id", "type", "actor.login", "created_at",
	},

	// ── Security ──────────────────────────────────────────────────────
	"github_list_code_scanning_alerts": {
		"number", "state", "rule.id", "rule.severity",
		"most_recent_instance.ref", "html_url", "created_at",
	},
	"github_list_secret_scanning_alerts": {
		"number", "state", "secret_type", "html_url", "created_at",
	},
	"github_list_dependabot_alerts": {
		"number", "state", "dependency.package.name",
		"security_advisory.severity", "html_url", "created_at",
	},

	// ── Webhooks ──────────────────────────────────────────────────────
	"github_list_hooks":       {"id", "name", "active", "config.url", "events", "updated_at"},
	"github_list_deploy_keys": {"id", "title", "key", "read_only", "created_at"},
}

// fieldCompactionSpecs holds pre-parsed CompactFields, built once at package init.
// If any spec is invalid, the program panics at startup (fail-fast).
var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("github: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
