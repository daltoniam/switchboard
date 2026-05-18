package github

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Repositories ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("github_search_repos"), Description: "Search GitHub repositories. Start here to find repos by name, topic, or language.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_repo"), Description: "Get a repository by owner/name. Use after search_repos or when you already know the owner/repo.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_user_repos"), Description: "List repositories for a user. Use when you know the username; prefer search_repos for keyword discovery.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username", Required: true}, {Name: mcp.ParamName("type"), Description: "Type: all, owner, member (default: owner)"}, {Name: mcp.ParamName("sort"), Description: "Sort: created, updated, pushed, full_name"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_org_repos"), Description: "List repositories for an organization. Use when you know the org; prefer search_repos for keyword discovery.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("type"), Description: "Type: all, public, private, forks, sources, member"}, {Name: mcp.ParamName("sort"), Description: "Sort: created, updated, pushed, full_name"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_repo"), Description: "Create a repository for the authenticated user or an org",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("org"), Description: "Organization (omit for user repo)"}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("private"), Description: "Private repo (true/false)"}, {Name: mcp.ParamName("auto_init"), Description: "Initialize with README (true/false)"}},
	},
	{
		Name: mcp.ToolName("github_delete_repo"), Description: "Delete a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_branches"), Description: "List branches of a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_branch"), Description: "Get a specific branch",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("branch"), Description: "Branch name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_tags"), Description: "List tags of a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_contributors"), Description: "List contributors to a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_languages"), Description: "List languages used in a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_topics"), Description: "List repository topics",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_readme"), Description: "Get the README for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (branch/tag/sha)"}},
	},
	{
		Name: mcp.ToolName("github_get_file_contents"), Description: "Get file or directory contents from a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("path"), Description: "File path", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (branch/tag/sha)"}},
	},
	{
		Name: mcp.ToolName("github_create_update_file"), Description: "Create or update a file in a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("path"), Description: "File path", Required: true}, {Name: mcp.ParamName("message"), Description: "Commit message", Required: true}, {Name: mcp.ParamName("content"), Description: "Raw file content (plain text, not base64 — encoding is handled automatically)", Required: true}, {Name: mcp.ParamName("sha"), Description: "SHA of file being replaced (required for update)"}, {Name: mcp.ParamName("branch"), Description: "Target branch"}},
	},
	{
		Name: mcp.ToolName("github_delete_file"), Description: "Delete a file from a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("path"), Description: "File path", Required: true}, {Name: mcp.ParamName("message"), Description: "Commit message", Required: true}, {Name: mcp.ParamName("sha"), Description: "SHA of file to delete", Required: true}, {Name: mcp.ParamName("branch"), Description: "Target branch"}},
	},
	{
		Name: mcp.ToolName("github_list_forks"), Description: "List forks of a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sort"), Description: "Sort: newest, oldest, stargazers, watchers"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_fork"), Description: "Fork a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("organization"), Description: "Organization to fork into"}},
	},
	{
		Name: mcp.ToolName("github_list_collaborators"), Description: "List collaborators on a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_commit_activity"), Description: "Get the last year of commit activity (weekly)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_repo_teams"), Description: "List teams with access to a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_compare_commits"), Description: "Compare two commits, branches, or tags. Use to see what changed between refs (commit list and diff). Start here for 'what changed in prod' queries.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("base"), Description: "Base ref (branch, tag, or SHA)", Required: true}, {Name: mcp.ParamName("head"), Description: "Head ref (branch, tag, or SHA)", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_merge_upstream"), Description: "Sync a fork branch with the upstream repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("branch"), Description: "Branch to sync", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_autolinks"), Description: "List autolink references for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name",

		// ── Repositories (extended) ───────────────────────────────────────
		Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}},
	},

	{
		Name: mcp.ToolName("github_edit_repo"), Description: "Update a repository's settings (description, visibility, default branch, etc.)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("homepage"), Description: "Homepage URL"}, {Name: mcp.ParamName("default_branch"), Description: "Default branch name"}, {Name: mcp.ParamName("private"), Description: "Private (true/false)"}, {Name: mcp.ParamName("archived"), Description: "Archived (true/false)"}, {Name: mcp.ParamName("has_issues"), Description: "Enable issues (true/false)"}, {Name: mcp.ParamName("has_projects"), Description: "Enable projects (true/false)"}, {Name: mcp.ParamName("has_wiki"), Description: "Enable wiki (true/false)"}},
	},
	{
		Name: mcp.ToolName("github_replace_topics"), Description: "Replace all topics on a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("topics"), Description: "Comma-separated topic names", Required: true}},
	},
	{
		Name: mcp.ToolName("github_rename_branch"), Description: "Rename a branch",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("branch"), Description: "Current branch name", Required: true}, {Name: mcp.ParamName("new_name"), Description: "New branch name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_add_collaborator"), Description: "Add a collaborator to a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("username"), Description: "User to add", Required: true}, {Name: mcp.ParamName("permission"), Description: "Permission: pull, triage, push, maintain, admin"}},
	},
	{
		Name: mcp.ToolName("github_remove_collaborator"), Description: "Remove a collaborator from a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("username"), Description: "User to remove", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_combined_status"), Description: "Get the combined commit status for a ref (aggregates all status checks)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (SHA, branch, or tag)", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_statuses"), Description: "List commit statuses for a ref",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (SHA, branch, or tag)", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_status"), Description: "Create a commit status",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sha"), Description: "Commit SHA", Required: true}, {Name: mcp.ParamName("state"), Description: "State: error, failure, pending, success", Required: true}, {Name: mcp.ParamName("target_url"), Description: "URL to associate with status"}, {Name: mcp.ParamName("description"), Description: "Short description"}, {Name: mcp.ParamName("context"), Description: "Status context identifier"}},
	},
	{
		Name: mcp.ToolName("github_list_deployments"), Description: "List deployments for a repository. Start here for deploy status, recent deploys, and rollout history.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("environment"), Description: "Filter by environment"}, {Name: mcp.ParamName("ref"), Description: "Filter by ref"}, {Name: mcp.ParamName("task"), Description: "Filter by task"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_deployment"), Description: "Get a single deployment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("deployment_id"), Description: "Deployment ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_deployment"), Description: "Create a deployment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Ref to deploy (branch, tag, or SHA)", Required: true}, {Name: mcp.ParamName("task"), Description: "Task (default: deploy)"}, {Name: mcp.ParamName("environment"), Description: "Environment name"}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("auto_merge"), Description: "Auto-merge default branch into ref (true/false)"}},
	},
	{
		Name: mcp.ToolName("github_list_deployment_statuses"), Description: "List statuses for a deployment. Use after list_deployments to check deploy progress or failure.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("deployment_id"), Description: "Deployment ID", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_deployment_status"), Description: "Create a deployment status",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("deployment_id"), Description: "Deployment ID", Required: true}, {Name: mcp.ParamName("state"), Description: "State: error, failure, inactive, in_progress, queued, pending, success", Required: true}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("log_url"), Description: "Log URL"}, {Name: mcp.ParamName("environment"), Description: "Override environment name"}},
	},
	{
		Name: mcp.ToolName("github_list_environments"), Description: "List environments for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_environment"), Description: "Get a single environment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("environment"), Description: "Environment name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_branch_protection"), Description: "Get branch protection rules",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("branch"), Description: "Branch name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_remove_branch_protection"), Description: "Remove branch protection rules",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("branch"), Description: "Branch name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_rulesets"), Description: "List repository rulesets",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_ruleset"), Description: "Get a repository ruleset by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ruleset_id"), Description: "Ruleset ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_rules_for_branch"), Description: "Get active rules that apply to a branch",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("branch"), Description: "Branch name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_traffic_views"), Description: "Get repository traffic page views (last 14 days, push access required)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("per"), Description: "Aggregation period: day, week"}},
	},
	{
		Name: mcp.ToolName("github_list_traffic_clones"), Description: "Get repository traffic clones (last 14 days, push access required)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("per"), Description: "Aggregation period: day, week"}},
	},
	{
		Name: mcp.ToolName("github_list_traffic_referrers"), Description: "Get top referral sources for a repository (push access required)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_traffic_paths"), Description: "Get popular content paths for a repository (push access required)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_community_health"), Description: "Get community health metrics for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_dispatch_event"), Description: "Trigger a repository dispatch event",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("event_type"), Description: "Custom event type string", Required: true}},
	},
	{
		Name: mcp.ToolName("github_merge_branch"), Description: "Merge a branch into another (not a PR merge — use github_merge_pull for PRs)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("base"), Description: "Branch to merge into", Required: true}, {Name: mcp.ParamName("head"), Description: "Branch to merge from", Required: true}, {Name: mcp.ParamName("commit_message"), Description: "Merge commit message"}},
	},
	{
		Name: mcp.ToolName("github_edit_release"), Description: "Update a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("release_id"), Description: "Release ID", Required: true}, {Name: mcp.ParamName("tag_name"), Description: "New tag name"}, {Name: mcp.ParamName("name"), Description: "New release name"}, {Name: mcp.ParamName("body"), Description: "New release notes"}, {Name: mcp.ParamName("draft"), Description: "Draft (true/false)"}, {Name: mcp.ParamName("prerelease"), Description: "Pre-release (true/false)"}},
	},
	{
		Name: mcp.ToolName("github_generate_release_notes"), Description: "Auto-generate release notes content between two tags",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("tag_name"), Description: "Tag for the release", Required: true}, {Name: mcp.ParamName("previous_tag_name"), Description: "Previous tag to compare against"}, {Name: mcp.ParamName("target_commitish"), Description: "Branch or SHA to tag"}},
	},
	{
		Name: mcp.ToolName("github_list_commit_comments"), Description: "List comments on a specific commit",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sha"), Description: "Commit SHA", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_commit_comment"), Description: "Create a comment on a commit",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sha"), Description: "Commit SHA", Required: true}, {Name: mcp.ParamName("body"),

		// ── Issues ────────────────────────────────────────────────────────
		Description: "Comment body", Required: true}, {Name: mcp.ParamName("path"), Description: "Relative file path"}, {Name: mcp.ParamName("position"), Description: "Line position in the diff"}},
	},

	{
		Name: mcp.ToolName("github_list_issues"), Description: "List issues for a repository. Start here for issue workflows when you know the repo. For cross-repo search, use search_issues.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("state"), Description: "State: open, closed, all"}, {Name: mcp.ParamName("labels"), Description: "Comma-separated label names"}, {Name: mcp.ParamName("sort"), Description: "Sort: created, updated, comments"}, {Name: mcp.ParamName("direction"), Description: "Direction: asc, desc"}, {Name: mcp.ParamName("assignee"), Description: "Filter by assignee username"}, {Name: mcp.ParamName("milestone"), Description: "Milestone number"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_issue"), Description: "Get a single issue with full details. Use after list_issues or search_issues to drill into a specific issue.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_issue"), Description: "Create an issue. Requires owner, repo, and title at minimum.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("title"), Description: "Issue title", Required: true}, {Name: mcp.ParamName("body"), Description: "Issue body (markdown)"}, {Name: mcp.ParamName("assignees"), Description: "Comma-separated assignee usernames"}, {Name: mcp.ParamName("labels"), Description: "Comma-separated label names"}, {Name: mcp.ParamName("milestone"), Description: "Milestone number"}},
	},
	{
		Name: mcp.ToolName("github_update_issue"), Description: "Update an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("title"), Description: "New title"}, {Name: mcp.ParamName("body"), Description: "New body"}, {Name: mcp.ParamName("state"), Description: "State: open, closed"}, {Name: mcp.ParamName("assignees"), Description: "Comma-separated assignee usernames"}, {Name: mcp.ParamName("labels"), Description: "Comma-separated label names"}, {Name: mcp.ParamName("milestone"), Description: "Milestone number"}},
	},
	{
		Name: mcp.ToolName("github_list_issue_comments"), Description: "List comments on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_issue_comment"), Description: "Create a comment on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("body"), Description: "Comment body (markdown)", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_issue_labels"), Description: "List labels on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}},
	},
	{
		Name: mcp.ToolName("github_add_issue_labels"), Description: "Add labels to an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("labels"), Description: "Comma-separated label names to add", Required: true}},
	},
	{
		Name: mcp.ToolName("github_remove_issue_label"), Description: "Remove a label from an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("label"), Description: "Label name to remove", Required: true}},
	},
	{
		Name: mcp.ToolName("github_lock_issue"), Description: "Lock an issue conversation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("lock_reason"), Description: "Reason: off-topic, too heated, resolved, spam"}},
	},
	{
		Name: mcp.ToolName("github_unlock_issue"), Description: "Unlock an issue conversation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_milestones"), Description: "List milestones for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("state"), Description: "State: open, closed, all"}, {Name: mcp.ParamName("sort"), Description: "Sort: due_on, completeness"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_milestone"), Description: "Create a milestone",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("title"), Description: "Milestone title", Required: true}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("due_on"), Description: "Due date (ISO 8601 YYYY-MM-DDT00:00:00Z)"}, {Name: mcp.ParamName("state"), Description: "State: open, closed"}},
	},
	{
		Name: mcp.ToolName("github_list_issue_events"), Description: "List events on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_issue_timeline"), Description: "List timeline events for an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_assignees"), Description: "List available assignees for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName(

		// ── Issues (extended) ─────────────────────────────────────────────
		"page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_update_issue_comment"), Description: "Edit an issue or PR comment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}, {Name: mcp.ParamName("body"), Description: "New comment body (markdown)", Required: true}},
	},
	{
		Name: mcp.ToolName("github_delete_issue_comment"), Description: "Delete an issue or PR comment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_update_milestone"), Description: "Update a milestone",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Milestone number", Required: true}, {Name: mcp.ParamName("title"), Description: "New title"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("state"), Description: "State: open, closed"}, {Name: mcp.ParamName("due_on"), Description: "Due date (ISO 8601)"}},
	},
	{
		Name: mcp.ToolName("github_delete_milestone"), Description: "Delete a milestone",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Milestone number", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_labels"), Description: "List all labels in a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_label"), Description: "Create a label in a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("name"), Description: "Label name", Required: true}, {Name: mcp.ParamName("color"), Description: "Color hex code (without #)", Required: true}, {Name: mcp.ParamName("description"), Description: "Label description"}},
	},
	{
		Name: mcp.ToolName("github_edit_label"), Description: "Update a label",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("name"), Description: "Current label name", Required: true}, {Name: mcp.ParamName("new_name"), Description: "New label name"}, {Name: mcp.ParamName("color"), Description: "New color hex code (without #)"}, {Name: mcp.ParamName("description"), Description: "New description"}},
	},
	{
		Name: mcp.ToolName("github_delete_label"), Description: "Delete a label from a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("name"), Description: "Label name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_issue_reaction"), Description: "Add a reaction to an issue (+1, -1, laugh, confused, heart, hooray, rocket, eyes)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number", Required: true}, {Name: mcp.ParamName("content"), Description: "Reaction: +1, -1, laugh, confused, heart, hooray, rocket, eyes", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_issue_comment_reaction"), Description: "Add a reaction to an issue comment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}, {Name: mcp.ParamName("content"), Description: "Reaction: +1, -1, laugh, confused, heart, hooray, rocket, eyes", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_issue_reactions"), Description: "List reactions on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("number"), Description: "Issue number",

		// ── Pull Requests ─────────────────────────────────────────────────
		Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_pulls"), Description: "List pull requests for a repository. Start here for PR workflows when you know the repo. For cross-repo search, use search_issues with type:pr.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("state"), Description: "State: open, closed, all"}, {Name: mcp.ParamName("head"), Description: "Filter by head user:branch"}, {Name: mcp.ParamName("base"), Description: "Filter by base branch"}, {Name: mcp.ParamName("sort"), Description: "Sort: created, updated, popularity, long-running"}, {Name: mcp.ParamName("direction"), Description: "Direction: asc, desc"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_pull"), Description: "Get a single pull request with full details. Use after list_pulls to drill into a specific PR. For the diff, follow up with get_pull_diff.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_pull_diff"), Description: "Get the raw unified diff of a pull request. Use after get_pull for the full code diff.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_pull"), Description: "Create a pull request. Requires head branch and base branch.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("title"), Description: "PR title", Required: true}, {Name: mcp.ParamName("head"), Description: "Head branch (or user:branch for cross-repo)", Required: true}, {Name: mcp.ParamName("base"), Description: "Base branch", Required: true}, {Name: mcp.ParamName("body"), Description: "PR body (markdown)"}, {Name: mcp.ParamName("draft"), Description: "Create as draft (true/false)"}},
	},
	{
		Name: mcp.ToolName("github_update_pull"), Description: "Update a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("title"), Description: "New title"}, {Name: mcp.ParamName("body"), Description: "New body"}, {Name: mcp.ParamName("state"), Description: "State: open, closed"}, {Name: mcp.ParamName("base"), Description: "New base branch"}},
	},
	{
		Name: mcp.ToolName("github_list_pull_commits"), Description: "List commits on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_pull_files"), Description: "List files changed in a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_pull_reviews"), Description: "List reviews on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_pull_review"), Description: "Create a review on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("body"), Description: "Review body"}, {Name: mcp.ParamName("event"), Description: "Review action: APPROVE, REQUEST_CHANGES, COMMENT", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_pull_comments"), Description: "List review comments on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_pull_comment"), Description: "Create a review comment on a pull request diff",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("body"), Description: "Comment body", Required: true}, {Name: mcp.ParamName("commit_id"), Description: "SHA of the commit to comment on", Required: true}, {Name: mcp.ParamName("path"), Description: "Relative file path", Required: true}, {Name: mcp.ParamName("line"), Description: "Line number in the diff"}},
	},
	{
		Name: mcp.ToolName("github_get_pull_comment"), Description: "Get a single review comment on a pull request by its comment ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_reply_to_pull_comment"), Description: "Reply to an existing review comment thread on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("body"), Description: "Reply body", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "ID of the comment to reply to", Required: true}},
	},
	{
		Name: mcp.ToolName("github_update_pull_comment"), Description: "Update the body of a review comment on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}, {Name: mcp.ParamName("body"), Description: "New comment body (markdown)", Required: true}},
	},
	{
		Name: mcp.ToolName("github_delete_pull_comment"), Description: "Delete a review comment on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_merge_pull"), Description: "Merge a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("commit_message"), Description: "Merge commit message"}, {Name: mcp.ParamName("merge_method"), Description: "Method: merge, squash, rebase"}},
	},
	{
		Name: mcp.ToolName("github_list_requested_reviewers"), Description: "List requested reviewers on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}},
	},
	{
		Name: mcp.ToolName("github_request_reviewers"), Description: "Request reviewers on a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName(

		// ── Pull Requests (extended) ──────────────────────────────────────
		"reviewers"), Description: "Comma-separated usernames"}, {Name: mcp.ParamName("team_reviewers"), Description: "Comma-separated team slugs"}},
	},

	{
		Name: mcp.ToolName("github_dismiss_pull_review"), Description: "Dismiss a pull request review",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("review_id"), Description: "Review ID", Required: true}, {Name: mcp.ParamName("message"), Description: "Dismissal message", Required: true}},
	},
	{
		Name: mcp.ToolName("github_update_pull_branch"), Description: "Update a PR branch with the latest changes from the base branch",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("expected_head_sha"), Description: "Expected SHA of the PR head (for optimistic locking)"}},
	},
	{
		Name: mcp.ToolName("github_remove_reviewers"), Description: "Remove requested reviewers from a pull request",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("pull_number"), Description: "Pull request number", Required: true}, {Name: mcp.ParamName("reviewers"), Description: "Comma-separated usernames"}, {Name: mcp.ParamName("team_reviewers"), Description: "Comma-separated team slugs"}},
	},
	{
		Name: mcp.ToolName("github_list_pulls_with_commit"), Description: "List pull requests that contain a specific commit",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sha"), Description: "Commit SHA",

		// ── Git (low-level) ───────────────────────────────────────────────
		Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_get_commit"), Description: "Get a commit by SHA including files changed. For comparing two refs, use compare_commits instead.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sha"), Description: "Commit SHA", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_commits"), Description: "List commits on a branch",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sha"), Description: "Branch name or SHA to start listing from"}, {Name: mcp.ParamName("path"), Description: "Only commits containing this file path"}, {Name: mcp.ParamName("author"), Description: "GitHub login or email to filter by"}, {Name: mcp.ParamName("since"), Description: "Only commits after this date (ISO 8601)"}, {Name: mcp.ParamName("until"), Description: "Only commits before this date (ISO 8601)"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_ref"), Description: "Get a git reference (branch or tag)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Reference (e.g., heads/main, tags/v1.0)", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_ref"), Description: "Create a git reference (branch or tag)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Full reference path (e.g., refs/heads/new-branch)", Required: true}, {Name: mcp.ParamName("sha"), Description: "SHA to point the ref at", Required: true}},
	},
	{
		Name: mcp.ToolName("github_delete_ref"), Description: "Delete a git reference",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Reference to delete (e.g., heads/old-branch)", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_tree"), Description: "Get a git tree (directory listing)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("sha"), Description: "Tree SHA or branch name", Required: true}, {Name: mcp.ParamName("recursive"), Description: "Recurse into subtrees (true/false)"}},
	},
	{
		Name: mcp.ToolName("github_create_tag"), Description: "Create an annotated tag object",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("tag"), Description: "Tag name", Required: true}, {Name: mcp.ParamName("message"), Description:

		// ── Users ─────────────────────────────────────────────────────────
		"Tag message", Required: true}, {Name: mcp.ParamName("sha"), Description: "SHA of object to tag", Required: true}, {Name: mcp.ParamName("type"), Description: "Object type: commit, tree, blob"}},
	},

	{
		Name: mcp.ToolName("github_get_authenticated_user"), Description: "Get the currently authenticated user",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("github_get_user"), Description: "Get a user by username",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_user_followers"), Description: "List followers of a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_user_following"), Description: "List users that a user follows",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_user_keys"), Description: "List public SSH keys for a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {

		// ── Organizations ─────────────────────────────────────────────────
		Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_get_org"), Description: "Get an organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization login name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_user_orgs"), Description: "List organizations for a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username (empty for authenticated user)"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_org_members"), Description: "List organization members",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("role"), Description: "Filter: all, admin, member"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_org_teams"), Description: "List teams in an organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_team_by_slug"), Description: "Get a team by slug",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_team_members"), Description: "List members of a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("role"), Description: "Filter: all, member, maintainer"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_team_repos"), Description: "List repositories for a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}, {Name: mcp.ParamName(

		// ── Teams (extended) ──────────────────────────────────────────────
		"page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_create_team"), Description: "Create a team in an organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("name"), Description: "Team name", Required: true}, {Name: mcp.ParamName("description"), Description: "Team description"}, {Name: mcp.ParamName("privacy"), Description: "Privacy: secret, closed"}, {Name: mcp.ParamName("permission"), Description: "Default permission: pull, push"}},
	},
	{
		Name: mcp.ToolName("github_edit_team"), Description: "Update a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("name"), Description: "New team name", Required: true}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("privacy"), Description: "Privacy: secret, closed"}, {Name: mcp.ParamName("permission"), Description: "Default permission: pull, push"}},
	},
	{
		Name: mcp.ToolName("github_delete_team"), Description: "Delete a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}},
	},
	{
		Name: mcp.ToolName("github_add_team_member"), Description: "Add or update a user's membership in a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("username"), Description: "GitHub username", Required: true}, {Name: mcp.ParamName("role"), Description: "Role: member, maintainer"}},
	},
	{
		Name: mcp.ToolName("github_remove_team_member"), Description: "Remove a user from a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("username"), Description: "GitHub username", Required: true}},
	},
	{
		Name: mcp.ToolName("github_add_team_repo"), Description: "Add a repository to a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("permission"), Description: "Permission: pull, triage, push, maintain, admin"}},
	},
	{
		Name: mcp.ToolName("github_remove_team_repo"), Description: "Remove a repository from a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_pending_org_invitations"), Description: "List pending organization invitations",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_outside_collaborators"), Description: "List outside collaborators for an organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("filter"), Description: "Filter: 2fa_disabled, all"}, {Name: mcp.ParamName(

		// ── Actions (CI/CD) ───────────────────────────────────────────────
		"page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_workflows"), Description: "List workflows in a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_workflow_runs"), Description: "List workflow runs for a repository. Use to check CI status or recent builds.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("workflow_id"), Description: "Workflow ID or filename (e.g., ci.yml)"}, {Name: mcp.ParamName("branch"), Description: "Filter by branch"}, {Name: mcp.ParamName("event"), Description: "Filter by event (push, pull_request, etc.)"}, {Name: mcp.ParamName("status"), Description: "Filter: completed, action_required, cancelled, failure, neutral, skipped, stale, success, timed_out, in_progress, queued, requested, waiting, pending"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_workflow_run"), Description: "Get a specific workflow run. Use after list_workflow_runs for full run details.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("run_id"), Description: "Workflow run ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_workflow_jobs"), Description: "List jobs for a workflow run",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("run_id"), Description: "Workflow run ID", Required: true}, {Name: mcp.ParamName("filter"), Description: "Filter: latest, all"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_download_workflow_logs"), Description: "Get a URL to download workflow run logs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("run_id"), Description: "Workflow run ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_rerun_workflow"), Description: "Re-run a workflow run",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("run_id"), Description: "Workflow run ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_cancel_workflow_run"), Description: "Cancel a workflow run",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("run_id"), Description: "Workflow run ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_repo_secrets"), Description: "List repository Actions secrets (names only, not values)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_artifacts"), Description: "List artifacts for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_environment_secrets"), Description: "List secrets for an environment (names only)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("environment"), Description: "Environment name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_org_secrets"), Description: "List organization Actions secrets (names only)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"},

		// ── Actions (extended) ────────────────────────────────────────────
		{Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_trigger_workflow"), Description: "Trigger a workflow dispatch event",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("workflow_id"), Description: "Workflow filename (e.g., ci.yml)", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref to run workflow on (branch or tag)", Required: true}},
	},
	{
		Name: mcp.ToolName("github_rerun_failed_jobs"), Description: "Re-run only the failed jobs of a workflow run",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("run_id"), Description: "Workflow run ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_workflow_job"), Description: "Get a single workflow job by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("job_id"), Description: "Job ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_workflow_job_logs"), Description: "Get a URL to download a single job's logs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("job_id"), Description: "Job ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_delete_workflow_run"), Description: "Delete a workflow run",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("run_id"), Description: "Workflow run ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_repo_variables"), Description: "List repository Actions variables (names and values)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_repo_variable"), Description: "Create a repository Actions variable",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("name"), Description: "Variable name", Required: true}, {Name: mcp.ParamName("value"), Description: "Variable value", Required: true}},
	},
	{
		Name: mcp.ToolName("github_update_repo_variable"), Description: "Update a repository Actions variable",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("name"), Description: "Variable name", Required: true}, {Name: mcp.ParamName("value"), Description: "New variable value", Required: true}},
	},
	{
		Name: mcp.ToolName("github_delete_repo_variable"), Description: "Delete a repository Actions variable",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("name"), Description: "Variable name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_org_variables"), Description: "List organization Actions variables (names and values)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_env_variables"), Description: "List environment Actions variables (names and values)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("environment"), Description: "Environment name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_runners"), Description: "List self-hosted runners for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_org_runners"), Description: "List self-hosted runners for an organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"},

		// ── Checks ────────────────────────────────────────────────────────
		{Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_check_runs"), Description: "List check runs for a git reference",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (SHA, branch, or tag)", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_check_run"), Description: "Get a check run by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("check_run_id"), Description: "Check run ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_check_suites"), Description: "List check suites for a git reference",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (SHA, branch, or tag)",

		// ── Releases ──────────────────────────────────────────────────────
		Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_releases"), Description: "List releases for a repository. Start here for release history, versioning, and what shipped.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_release"), Description: "Get a release by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("release_id"), Description: "Release ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_get_latest_release"), Description: "Get the latest release for a repository. Use to find what version is current or what shipped most recently.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_release"), Description: "Create a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("tag_name"), Description: "Tag name for the release", Required: true}, {Name: mcp.ParamName("name"), Description: "Release name"}, {Name: mcp.ParamName("body"), Description: "Release notes (markdown)"}, {Name: mcp.ParamName("draft"), Description: "Create as draft (true/false)"}, {Name: mcp.ParamName("prerelease"), Description: "Mark as pre-release (true/false)"}, {Name: mcp.ParamName("target_commitish"), Description: "Branch or SHA to tag (defaults to default branch)"}},
	},
	{
		Name: mcp.ToolName("github_delete_release"), Description: "Delete a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("release_id"), Description: "Release ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_release_assets"), Description: "List assets for a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("release_id"), Description: "Release ID",

		// ── Gists ─────────────────────────────────────────────────────────
		Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_gists"), Description: "List gists for the authenticated user or a specific user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username (empty for authenticated user)"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_gist"), Description: "Get a gist by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Gist ID", Required: true}},
	},
	{
		Name: mcp.ToolName("github_create_gist"), Description: "Create a gist",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("description"), Description: "Gist description"}, {Name: mcp.ParamName("public"), Description: "Public gist (true/false)"}, {Name: mcp.ParamName("filename"), Description: "Filename for the gist content",

		// ── Search ────────────────────────────────────────────────────────
		Required: true}, {Name: mcp.ParamName("content"), Description: "File content", Required: true}},
	},

	{
		Name: mcp.ToolName("github_search_topics"), Description: "Search GitHub topics",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_search_labels"), Description: "Search labels in a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("repository_id"), Description: "Repository numeric ID", Required: true}, {Name: mcp.ParamName("query"), Description: "Search query", Required: true}, {Name: mcp.ParamName("sort"), Description: "Sort: created, updated"}, {Name: mcp.ParamName("order"), Description: "Order: asc, desc"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_search_code"), Description: "Search code across GitHub repositories. Start here to find files, functions, or strings across repos.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (supports qualifiers like language:go, repo:owner/name)", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_search_issues"), Description: "Search issues and pull requests across GitHub. Start here for cross-repo issue/PR discovery. Add is:pr or is:issue to filter. Use for cross-repo bug, ticket, or PR discovery.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (supports qualifiers like is:issue, is:pr, repo:owner/name, state:open)", Required: true}, {Name: mcp.ParamName("sort"), Description: "Sort: comments, reactions, created, updated"}, {Name: mcp.ParamName("order"), Description: "Order: asc, desc"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_search_users"), Description: "Search GitHub users",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (supports qualifiers like location:, language:, followers:>N)", Required: true}, {Name: mcp.ParamName("sort"), Description: "Sort: followers, repositories, joined"}, {Name: mcp.ParamName("order"), Description: "Order: asc, desc"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_search_commits"), Description: "Search commits across GitHub",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (supports qualifiers like author:, repo:, committer:)", Required: true}, {Name: mcp.ParamName("sort"), Description: "Sort: author-date, committer-date"}, {Name: mcp.ParamName("order"), Description: "Order: asc, desc"},

		// ── Activity ──────────────────────────────────────────────────────
		{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_stargazers"), Description: "List stargazers for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_watchers"), Description: "List watchers (subscribers) of a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_notifications"), Description: "List notifications for the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("all"), Description: "Show all notifications including read (true/false)"}, {Name: mcp.ParamName("participating"), Description: "Only show participating notifications (true/false)"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_list_repo_events"), Description: "List events for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName(

		// ── Activity (extended) ───────────────────────────────────────────
		"page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_mark_notifications_read"), Description: "Mark all notifications as read",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("github_star_repo"), Description: "Star a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_unstar_repo"), Description: "Unstar a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}},
	},
	{
		Name: mcp.ToolName("github_list_starred"), Description: "List repositories starred by a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("username"), Description: "GitHub username (empty for authenticated user)"}, {Name: mcp.ParamName("sort"), Description: "Sort: created, updated"}, {Name: mcp.ParamName("direction"), Description:

		// ── Code Scanning ─────────────────────────────────────────────────
		"Direction: asc, desc"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_code_scanning_alerts"), Description: "List code scanning (SAST) alerts for a repository. Start here for security vulnerabilities found by CodeQL or other analyzers.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("state"), Description: "State: open, closed, dismissed, fixed"}, {Name: mcp.ParamName("ref"), Description: "Git ref to filter by"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_get_code_scanning_alert"), Description: "Get a code scanning alert",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.

		// ── Secret Scanning ───────────────────────────────────────────────
		ParamName("alert_number"), Description: "Alert number", Required: true}},
	},

	{
		Name: mcp.ToolName("github_list_secret_scanning_alerts"), Description: "List secret scanning alerts for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("state"), Description: "State: open, resolved"}, {Name: mcp.ParamName("secret_type"),

		// ── Secret Scanning (extended) ────────────────────────────────────
		Description: "Filter by secret type"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_get_secret_scanning_alert"), Description: "Get a single secret scanning alert",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.

		// ── Dependabot ────────────────────────────────────────────────────
		ParamName("alert_number"), Description: "Alert number", Required: true}},
	},

	{
		Name: mcp.ToolName("github_list_dependabot_alerts"), Description: "List Dependabot dependency vulnerability alerts for a repository. Start here for CVE impact on dependencies.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("state"), Description: "State: auto_dismissed, dismissed, fixed, open"}, {Name: mcp.ParamName("severity"), Description:

		// ── Dependabot (extended) ─────────────────────────────────────────
		"Severity: low, medium, high, critical"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_get_dependabot_alert"), Description: "Get a single Dependabot alert",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.

		// ── Code Scanning (extended) ─────────────────────────────────────
		ParamName("alert_number"), Description: "Alert number", Required: true}},
	},

	{
		Name: mcp.ToolName("github_list_code_scanning_analyses"), Description: "List code scanning SARIF analyses for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref to filter by"},

		// ── SBOM ─────────────────────────────────────────────────────────
		{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_get_sbom"), Description: "Get the software bill of materials (SBOM) for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description:

		// ── Copilot ───────────────────────────────────────────────────────
		"Repository name", Required: true}},
	},

	{
		Name: mcp.ToolName("github_get_copilot_org_usage"), Description: "Get Copilot usage metrics for an organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org"), Description: "Organization name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"},

		// ── Webhooks ──────────────────────────────────────────────────────
		{Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_list_hooks"), Description: "List webhooks for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("github_create_hook"), Description: "Create a webhook for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName("url"), Description: "Webhook payload URL", Required: true}, {Name: mcp.ParamName("content_type"), Description: "Content type: json, form"}, {Name: mcp.ParamName("events"), Description: "Comma-separated events (push, pull_request, issues, etc.)"}, {Name: mcp.ParamName("active"), Description: "Active (true/false, default true)"}},
	},
	{
		Name: mcp.ToolName("github_delete_hook"), Description: "Delete a webhook",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true},

		// ── Deploy Keys ───────────────────────────────────────────────────
		{Name: mcp.ParamName("hook_id"), Description: "Webhook ID", Required: true}},
	},

	{
		Name: mcp.ToolName("github_list_deploy_keys"), Description: "List deploy keys for a repository",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("owner"), Description: "Repository owner", Required: true}, {Name: mcp.ParamName("repo"), Description: "Repository name", Required: true}, {Name: mcp.ParamName(

		// ── Rate Limit ────────────────────────────────────────────────────
		"page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},

	{
		Name: mcp.ToolName("github_get_rate_limit"), Description: "Get API rate limit status for the authenticated user",
		Parameters: []mcp.Parameter{},
	},
}
