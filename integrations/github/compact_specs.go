package github

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// ── Shared field slices ──────────────────────────────────────────────

var repoListFields = []string{
	"full_name", "description", "language", "stargazers_count",
	"html_url", "private", "archived", "default_branch", "updated_at",
}

var commitListFields = []string{
	"sha", "commit.message", "commit.author.name", "commit.author.date", "html_url",
}

var userListFields = []string{"login", "html_url", "type"}

var secretListFields = []string{"name", "created_at", "updated_at"}

// itemsPrefix wraps field specs with "items[]." for search result compaction.
// Search responses are {"total_count": N, "items": [...]}, so per-item specs
// need the items[] prefix to target elements inside the wrapper.
func itemsPrefix(fields []string) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = "items[]." + f
	}
	return out
}

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
var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Repositories ──────────────────────────────────────────────────
	mcp.ToolName("github_list_user_repos"): repoListFields,
	mcp.ToolName("github_list_org_repos"):  repoListFields,
	mcp.ToolName("github_list_branches"):   {"name", "commit.sha", "protected"},
	mcp.ToolName("github_list_tags"):       {"name", "commit.sha"},
	mcp.ToolName("github_list_contributors"): {
		"login", "contributions", "html_url", "type",
	},
	mcp.ToolName("github_list_forks"): {
		"full_name", "owner.login", "html_url", "created_at", "default_branch",
	},
	mcp.ToolName("github_list_collaborators"): {"login", "html_url", "type", "permissions"},
	mcp.ToolName("github_list_repo_teams"):    {"name", "slug", "permission"},
	mcp.ToolName("github_list_autolinks"):     {"id", "key_prefix", "url_template", "is_alphanumeric"},

	// ── Issues ────────────────────────────────────────────────────────
	mcp.ToolName("github_list_issues"): {
		"number", "title", "state", "html_url", "created_at", "updated_at",
		"comments", "user.login", "labels[].name", "assignees[].login",
		"milestone.title",
	},
	mcp.ToolName("github_list_issue_comments"): {
		"id", "body", "user.login", "created_at", "html_url",
	},
	mcp.ToolName("github_list_milestones"): {
		"number", "title", "state", "open_issues", "closed_issues",
		"due_on", "html_url",
	},
	mcp.ToolName("github_list_issue_events"): {
		"id", "event", "actor.login", "created_at", "label.name",
		"assignee.login", "milestone.title",
	},
	mcp.ToolName("github_list_issue_timeline"): {
		"id", "event", "actor.login", "created_at",
		"source.issue.number",
	},
	mcp.ToolName("github_list_assignees"): userListFields,

	// ── Pull Requests ─────────────────────────────────────────────────
	mcp.ToolName("github_list_pulls"): {
		"number", "title", "state", "html_url", "created_at", "updated_at",
		"comments", "draft", "merged", "user.login", "head.ref", "base.ref",
		"head.repo.full_name", "labels[].name",
		"assignees[].login", "requested_reviewers[].login",
	},
	mcp.ToolName("github_list_pull_commits"): commitListFields,
	mcp.ToolName("github_list_pull_files"): {
		"sha", "filename", "status", "additions", "deletions",
	},
	mcp.ToolName("github_list_pull_reviews"): {
		"id", "state", "body", "user.login", "submitted_at", "html_url",
	},
	mcp.ToolName("github_list_pull_comments"): {
		"id", "body", "user.login", "created_at", "path", "line", "html_url",
	},
	mcp.ToolName("github_get_pull_comment"): {
		"id", "body", "user.login", "path", "line", "side", "created_at", "updated_at",
		"html_url", "in_reply_to_id", "commit_id", "subject_type",
	},

	// ── Git (low-level) ───────────────────────────────────────────────
	mcp.ToolName("github_list_commits"): commitListFields,

	// ── Users ─────────────────────────────────────────────────────────
	mcp.ToolName("github_list_user_followers"): userListFields,
	mcp.ToolName("github_list_user_following"): userListFields,
	mcp.ToolName("github_list_user_keys"):      {"id", "key"},

	// ── Organizations ─────────────────────────────────────────────────
	mcp.ToolName("github_list_org_members"):  userListFields,
	mcp.ToolName("github_list_org_teams"):    {"name", "slug", "permission", "description"},
	mcp.ToolName("github_list_team_members"): userListFields,
	mcp.ToolName("github_list_team_repos"): {
		"full_name", "description", "language", "html_url", "private",
	},

	// ── Actions (CI/CD) ───────────────────────────────────────────────
	mcp.ToolName("github_list_workflows"): {
		"id", "name", "state", "path", "html_url",
	},
	mcp.ToolName("github_list_workflow_runs"): {
		"id", "name", "display_title", "status", "conclusion",
		"head_branch", "head_sha", "actor.login",
		"created_at", "html_url", "event", "run_number", "run_attempt",
	},
	mcp.ToolName("github_list_workflow_jobs"): {
		"id", "name", "status", "conclusion", "started_at",
		"completed_at", "html_url",
		"steps[].name", "steps[].conclusion",
	},
	mcp.ToolName("github_list_repo_secrets"):        secretListFields,
	mcp.ToolName("github_list_artifacts"):           {"id", "name", "size_in_bytes", "created_at", "expired"},
	mcp.ToolName("github_list_environment_secrets"): secretListFields,
	mcp.ToolName("github_list_org_secrets"):         secretListFields,

	// ── Checks ────────────────────────────────────────────────────────
	mcp.ToolName("github_list_check_runs"): {
		"id", "name", "status", "conclusion", "started_at",
		"completed_at", "html_url", "output.title",
	},
	mcp.ToolName("github_list_check_suites"): {
		"id", "app.slug", "status", "conclusion", "head_branch", "created_at",
	},

	// ── Releases ──────────────────────────────────────────────────────
	mcp.ToolName("github_list_releases"): {
		"id", "tag_name", "name", "draft", "prerelease",
		"created_at", "published_at", "html_url",
	},
	mcp.ToolName("github_list_release_assets"): {
		"id", "name", "size", "download_count", "browser_download_url",
		"created_at",
	},

	// ── Gists ─────────────────────────────────────────────────────────
	mcp.ToolName("github_list_gists"): {
		"id", "description", "html_url", "created_at", "updated_at", "public",
	},

	// ── Activity ──────────────────────────────────────────────────────
	mcp.ToolName("github_list_stargazers"): userListFields,
	mcp.ToolName("github_list_watchers"):   userListFields,
	mcp.ToolName("github_list_notifications"): {
		"id", "reason", "subject.title", "subject.type", "subject.url",
		"repository.full_name", "updated_at", "unread",
	},
	mcp.ToolName("github_list_repo_events"): {
		"id", "type", "actor.login", "created_at",
		"payload.action", "payload.ref",
		"payload.pull_request.number", "payload.issue.number",
	},

	// ── Security ──────────────────────────────────────────────────────
	mcp.ToolName("github_list_code_scanning_alerts"): {
		"number", "state", "rule.id", "rule.severity", "rule.description",
		"most_recent_instance.ref", "html_url", "created_at",
	},
	mcp.ToolName("github_list_secret_scanning_alerts"): {
		"number", "state", "secret_type", "html_url", "created_at",
	},
	mcp.ToolName("github_list_dependabot_alerts"): {
		"number", "state", "dependency.package.name",
		"security_advisory.severity", "html_url", "created_at",
	},

	// ── Search ───────────────────────────────────────────────────────
	// Search responses are {"total_count": N, "items": [...]}.
	// Items use items[].field prefix; nested arrays use items[].labels[].name.
	mcp.ToolName("github_search_repos"): append([]string{"total_count"}, itemsPrefix(repoListFields)...),
	mcp.ToolName("github_search_issues"): append([]string{"total_count"}, itemsPrefix([]string{
		"number", "title", "state", "html_url", "created_at", "updated_at",
		"comments", "user.login", "labels[].name", "assignees[].login",
		"milestone.title", "repository.full_name",
	})...),
	mcp.ToolName("github_search_code"): append([]string{"total_count"}, itemsPrefix([]string{
		"repository.full_name", "path", "name", "html_url",
	})...),
	mcp.ToolName("github_search_users"):   append([]string{"total_count"}, itemsPrefix(userListFields)...),
	mcp.ToolName("github_search_commits"): append([]string{"total_count"}, itemsPrefix(append(commitListFields, "repository.full_name"))...),

	// ── Webhooks ──────────────────────────────────────────────────────
	mcp.ToolName("github_list_hooks"):       {"id", "name", "active", "config.url", "events", "updated_at"},
	mcp.ToolName("github_list_deploy_keys"): {"id", "title", "key", "read_only", "created_at"},

	// ── Repositories (extended) ───────────────────────────────────────
	mcp.ToolName("github_list_statuses"): {
		"id", "state", "context", "description", "target_url", "created_at",
	},
	mcp.ToolName("github_list_deployments"): {
		"id", "ref", "environment", "task", "description",
		"creator.login", "created_at", "updated_at",
	},
	mcp.ToolName("github_list_deployment_statuses"): {
		"id", "state", "description", "environment",
		"creator.login", "created_at", "log_url",
	},
	mcp.ToolName("github_list_environments"): {
		"id", "name", "created_at", "updated_at",
	},
	mcp.ToolName("github_list_rulesets"): {
		"id", "name", "enforcement", "source_type", "source", "created_at",
	},
	mcp.ToolName("github_list_traffic_referrers"): {
		"referrer", "count", "uniques",
	},
	mcp.ToolName("github_list_traffic_paths"): {
		"path", "title", "count", "uniques",
	},
	mcp.ToolName("github_list_commit_comments"): {
		"id", "body", "user.login", "created_at", "path", "position", "html_url",
	},

	// ── Issues (extended) ─────────────────────────────────────────────
	mcp.ToolName("github_list_labels"): {
		"name", "color", "description",
	},
	mcp.ToolName("github_list_issue_reactions"): {
		"id", "content", "user.login", "created_at",
	},

	// ── Pull Requests (extended) ──────────────────────────────────────
	mcp.ToolName("github_list_pulls_with_commit"): {
		"number", "title", "state", "html_url", "created_at",
		"user.login", "head.ref", "base.ref",
	},

	// ── Actions (extended) ────────────────────────────────────────────
	mcp.ToolName("github_list_repo_variables"): {"name", "value", "created_at", "updated_at"},
	mcp.ToolName("github_list_org_variables"):  {"name", "value", "created_at", "updated_at"},
	mcp.ToolName("github_list_env_variables"):  {"name", "value", "created_at", "updated_at"},
	mcp.ToolName("github_list_runners"): {
		"id", "name", "os", "status", "busy", "labels[].name",
	},
	mcp.ToolName("github_list_org_runners"): {
		"id", "name", "os", "status", "busy", "labels[].name",
	},

	// ── Teams/Orgs (extended) ─────────────────────────────────────────
	mcp.ToolName("github_list_pending_org_invitations"): {
		"id", "login", "email", "role", "created_at",
	},
	mcp.ToolName("github_list_outside_collaborators"): userListFields,

	// ── Search (extended) ─────────────────────────────────────────────
	mcp.ToolName("github_search_topics"): append([]string{"total_count"}, itemsPrefix([]string{
		"name", "display_name", "short_description", "created_by",
	})...),
	mcp.ToolName("github_search_labels"): append([]string{"total_count"}, itemsPrefix([]string{
		"name", "color", "description",
	})...),

	// ── Activity (extended) ───────────────────────────────────────────
	mcp.ToolName("github_list_starred"): {
		"repo.full_name", "repo.description", "repo.language",
		"repo.stargazers_count", "repo.html_url", "starred_at",
	},

	// ── Security (extended) ───────────────────────────────────────────
	mcp.ToolName("github_list_code_scanning_analyses"): {
		"id", "ref", "commit_sha", "analysis_key", "tool.name",
		"created_at", "results_count", "rules_count",
	},
}

// fieldCompactionSpecs holds pre-parsed CompactFields, built once at package init.
// If any spec is invalid, the program panics at startup (fail-fast).
var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("github: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
