package github

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Repositories ──────────────────────────────────────────────────
	{
		Name: "github_search_repos", Description: "Search GitHub repositories. Start here to find repos by name, topic, or language.",
		Parameters: map[string]string{"query": "Search query", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},
	{
		Name: "github_get_repo", Description: "Get a repository by owner/name. Use after search_repos or when you already know the owner/repo.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_user_repos", Description: "List repositories for a user. Use when you know the username; prefer search_repos for keyword discovery.",
		Parameters: map[string]string{"username": "GitHub username", "type": "Type: all, owner, member (default: owner)", "sort": "Sort: created, updated, pushed, full_name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"username"},
	},
	{
		Name: "github_list_org_repos", Description: "List repositories for an organization. Use when you know the org; prefer search_repos for keyword discovery.",
		Parameters: map[string]string{"org": "Organization name", "type": "Type: all, public, private, forks, sources, member", "sort": "Sort: created, updated, pushed, full_name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},
	{
		Name: "github_create_repo", Description: "Create a repository for the authenticated user or an org",
		Parameters: map[string]string{"name": "Repository name", "org": "Organization (omit for user repo)", "description": "Description", "private": "Private repo (true/false)", "auto_init": "Initialize with README (true/false)"},
		Required:   []string{"name"},
	},
	{
		Name: "github_delete_repo", Description: "Delete a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_branches", Description: "List branches of a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_branch", Description: "Get a specific branch",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "branch": "Branch name"},
		Required:   []string{"owner", "repo", "branch"},
	},
	{
		Name: "github_list_tags", Description: "List tags of a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_contributors", Description: "List contributors to a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_languages", Description: "List languages used in a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_topics", Description: "List repository topics",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_readme", Description: "Get the README for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Git ref (branch/tag/sha)"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_file_contents", Description: "Get file or directory contents from a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "path": "File path", "ref": "Git ref (branch/tag/sha)"},
		Required:   []string{"owner", "repo", "path"},
	},
	{
		Name: "github_create_update_file", Description: "Create or update a file in a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "path": "File path", "message": "Commit message", "content": "Base64-encoded file content", "sha": "SHA of file being replaced (required for update)", "branch": "Target branch"},
		Required:   []string{"owner", "repo", "path", "message", "content"},
	},
	{
		Name: "github_delete_file", Description: "Delete a file from a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "path": "File path", "message": "Commit message", "sha": "SHA of file to delete", "branch": "Target branch"},
		Required:   []string{"owner", "repo", "path", "message", "sha"},
	},
	{
		Name: "github_list_forks", Description: "List forks of a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sort": "Sort: newest, oldest, stargazers, watchers", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_create_fork", Description: "Fork a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "organization": "Organization to fork into"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_collaborators", Description: "List collaborators on a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_commit_activity", Description: "Get the last year of commit activity (weekly)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_repo_teams", Description: "List teams with access to a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_compare_commits", Description: "Compare two commits, branches, or tags. Use to see what changed between refs (commit list and diff). Start here for 'what changed in prod' queries.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "base": "Base ref (branch, tag, or SHA)", "head": "Head ref (branch, tag, or SHA)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "base", "head"},
	},
	{
		Name: "github_merge_upstream", Description: "Sync a fork branch with the upstream repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "branch": "Branch to sync"},
		Required:   []string{"owner", "repo", "branch"},
	},
	{
		Name: "github_list_autolinks", Description: "List autolink references for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number"},
		Required:   []string{"owner", "repo"},
	},

	// ── Repositories (extended) ───────────────────────────────────────
	{
		Name: "github_edit_repo", Description: "Update a repository's settings (description, visibility, default branch, etc.)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "description": "New description", "homepage": "Homepage URL", "default_branch": "Default branch name", "private": "Private (true/false)", "archived": "Archived (true/false)", "has_issues": "Enable issues (true/false)", "has_projects": "Enable projects (true/false)", "has_wiki": "Enable wiki (true/false)"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_replace_topics", Description: "Replace all topics on a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "topics": "Comma-separated topic names"},
		Required:   []string{"owner", "repo", "topics"},
	},
	{
		Name: "github_rename_branch", Description: "Rename a branch",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "branch": "Current branch name", "new_name": "New branch name"},
		Required:   []string{"owner", "repo", "branch", "new_name"},
	},
	{
		Name: "github_add_collaborator", Description: "Add a collaborator to a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "username": "User to add", "permission": "Permission: pull, triage, push, maintain, admin"},
		Required:   []string{"owner", "repo", "username"},
	},
	{
		Name: "github_remove_collaborator", Description: "Remove a collaborator from a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "username": "User to remove"},
		Required:   []string{"owner", "repo", "username"},
	},
	{
		Name: "github_get_combined_status", Description: "Get the combined commit status for a ref (aggregates all status checks)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Git ref (SHA, branch, or tag)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "ref"},
	},
	{
		Name: "github_list_statuses", Description: "List commit statuses for a ref",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Git ref (SHA, branch, or tag)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "ref"},
	},
	{
		Name: "github_create_status", Description: "Create a commit status",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Commit SHA", "state": "State: error, failure, pending, success", "target_url": "URL to associate with status", "description": "Short description", "context": "Status context identifier"},
		Required:   []string{"owner", "repo", "sha", "state"},
	},
	{
		Name: "github_list_deployments", Description: "List deployments for a repository. Start here for deploy status, recent deploys, and rollout history.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "environment": "Filter by environment", "ref": "Filter by ref", "task": "Filter by task", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_deployment", Description: "Get a single deployment",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "deployment_id": "Deployment ID"},
		Required:   []string{"owner", "repo", "deployment_id"},
	},
	{
		Name: "github_create_deployment", Description: "Create a deployment",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Ref to deploy (branch, tag, or SHA)", "task": "Task (default: deploy)", "environment": "Environment name", "description": "Description", "auto_merge": "Auto-merge default branch into ref (true/false)"},
		Required:   []string{"owner", "repo", "ref"},
	},
	{
		Name: "github_list_deployment_statuses", Description: "List statuses for a deployment. Use after list_deployments to check deploy progress or failure.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "deployment_id": "Deployment ID", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "deployment_id"},
	},
	{
		Name: "github_create_deployment_status", Description: "Create a deployment status",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "deployment_id": "Deployment ID", "state": "State: error, failure, inactive, in_progress, queued, pending, success", "description": "Description", "log_url": "Log URL", "environment": "Override environment name"},
		Required:   []string{"owner", "repo", "deployment_id", "state"},
	},
	{
		Name: "github_list_environments", Description: "List environments for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_environment", Description: "Get a single environment",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "environment": "Environment name"},
		Required:   []string{"owner", "repo", "environment"},
	},
	{
		Name: "github_get_branch_protection", Description: "Get branch protection rules",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "branch": "Branch name"},
		Required:   []string{"owner", "repo", "branch"},
	},
	{
		Name: "github_remove_branch_protection", Description: "Remove branch protection rules",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "branch": "Branch name"},
		Required:   []string{"owner", "repo", "branch"},
	},
	{
		Name: "github_list_rulesets", Description: "List repository rulesets",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_ruleset", Description: "Get a repository ruleset by ID",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ruleset_id": "Ruleset ID"},
		Required:   []string{"owner", "repo", "ruleset_id"},
	},
	{
		Name: "github_get_rules_for_branch", Description: "Get active rules that apply to a branch",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "branch": "Branch name"},
		Required:   []string{"owner", "repo", "branch"},
	},
	{
		Name: "github_list_traffic_views", Description: "Get repository traffic page views (last 14 days, push access required)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "per": "Aggregation period: day, week"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_traffic_clones", Description: "Get repository traffic clones (last 14 days, push access required)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "per": "Aggregation period: day, week"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_traffic_referrers", Description: "Get top referral sources for a repository (push access required)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_traffic_paths", Description: "Get popular content paths for a repository (push access required)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_community_health", Description: "Get community health metrics for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_dispatch_event", Description: "Trigger a repository dispatch event",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "event_type": "Custom event type string"},
		Required:   []string{"owner", "repo", "event_type"},
	},
	{
		Name: "github_merge_branch", Description: "Merge a branch into another (not a PR merge — use github_merge_pull for PRs)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "base": "Branch to merge into", "head": "Branch to merge from", "commit_message": "Merge commit message"},
		Required:   []string{"owner", "repo", "base", "head"},
	},
	{
		Name: "github_edit_release", Description: "Update a release",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "release_id": "Release ID", "tag_name": "New tag name", "name": "New release name", "body": "New release notes", "draft": "Draft (true/false)", "prerelease": "Pre-release (true/false)"},
		Required:   []string{"owner", "repo", "release_id"},
	},
	{
		Name: "github_generate_release_notes", Description: "Auto-generate release notes content between two tags",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "tag_name": "Tag for the release", "previous_tag_name": "Previous tag to compare against", "target_commitish": "Branch or SHA to tag"},
		Required:   []string{"owner", "repo", "tag_name"},
	},
	{
		Name: "github_list_commit_comments", Description: "List comments on a specific commit",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Commit SHA", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "sha"},
	},
	{
		Name: "github_create_commit_comment", Description: "Create a comment on a commit",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Commit SHA", "body": "Comment body", "path": "Relative file path", "position": "Line position in the diff"},
		Required:   []string{"owner", "repo", "sha", "body"},
	},

	// ── Issues ────────────────────────────────────────────────────────
	{
		Name: "github_list_issues", Description: "List issues for a repository. Start here for issue workflows when you know the repo. For cross-repo search, use search_issues.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open, closed, all", "labels": "Comma-separated label names", "sort": "Sort: created, updated, comments", "direction": "Direction: asc, desc", "assignee": "Filter by assignee username", "milestone": "Milestone number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_issue", Description: "Get a single issue with full details. Use after list_issues or search_issues to drill into a specific issue.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_create_issue", Description: "Create an issue. Requires owner, repo, and title at minimum.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "title": "Issue title", "body": "Issue body (markdown)", "assignees": "Comma-separated assignee usernames", "labels": "Comma-separated label names", "milestone": "Milestone number"},
		Required:   []string{"owner", "repo", "title"},
	},
	{
		Name: "github_update_issue", Description: "Update an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "title": "New title", "body": "New body", "state": "State: open, closed", "assignees": "Comma-separated assignee usernames", "labels": "Comma-separated label names", "milestone": "Milestone number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_issue_comments", Description: "List comments on an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_create_issue_comment", Description: "Create a comment on an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "body": "Comment body (markdown)"},
		Required:   []string{"owner", "repo", "number", "body"},
	},
	{
		Name: "github_list_issue_labels", Description: "List labels on an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_add_issue_labels", Description: "Add labels to an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "labels": "Comma-separated label names to add"},
		Required:   []string{"owner", "repo", "number", "labels"},
	},
	{
		Name: "github_remove_issue_label", Description: "Remove a label from an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "label": "Label name to remove"},
		Required:   []string{"owner", "repo", "number", "label"},
	},
	{
		Name: "github_lock_issue", Description: "Lock an issue conversation",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "lock_reason": "Reason: off-topic, too heated, resolved, spam"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_unlock_issue", Description: "Unlock an issue conversation",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_milestones", Description: "List milestones for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open, closed, all", "sort": "Sort: due_on, completeness", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_create_milestone", Description: "Create a milestone",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "title": "Milestone title", "description": "Description", "due_on": "Due date (ISO 8601 YYYY-MM-DDT00:00:00Z)", "state": "State: open, closed"},
		Required:   []string{"owner", "repo", "title"},
	},
	{
		Name: "github_list_issue_events", Description: "List events on an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_issue_timeline", Description: "List timeline events for an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_assignees", Description: "List available assignees for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},

	// ── Issues (extended) ─────────────────────────────────────────────
	{
		Name: "github_update_issue_comment", Description: "Edit an issue or PR comment",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "comment_id": "Comment ID", "body": "New comment body (markdown)"},
		Required:   []string{"owner", "repo", "comment_id", "body"},
	},
	{
		Name: "github_delete_issue_comment", Description: "Delete an issue or PR comment",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "comment_id": "Comment ID"},
		Required:   []string{"owner", "repo", "comment_id"},
	},
	{
		Name: "github_update_milestone", Description: "Update a milestone",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Milestone number", "title": "New title", "description": "New description", "state": "State: open, closed", "due_on": "Due date (ISO 8601)"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_delete_milestone", Description: "Delete a milestone",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Milestone number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_labels", Description: "List all labels in a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_create_label", Description: "Create a label in a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "name": "Label name", "color": "Color hex code (without #)", "description": "Label description"},
		Required:   []string{"owner", "repo", "name", "color"},
	},
	{
		Name: "github_edit_label", Description: "Update a label",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "name": "Current label name", "new_name": "New label name", "color": "New color hex code (without #)", "description": "New description"},
		Required:   []string{"owner", "repo", "name"},
	},
	{
		Name: "github_delete_label", Description: "Delete a label from a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "name": "Label name"},
		Required:   []string{"owner", "repo", "name"},
	},
	{
		Name: "github_create_issue_reaction", Description: "Add a reaction to an issue (+1, -1, laugh, confused, heart, hooray, rocket, eyes)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "content": "Reaction: +1, -1, laugh, confused, heart, hooray, rocket, eyes"},
		Required:   []string{"owner", "repo", "number", "content"},
	},
	{
		Name: "github_create_issue_comment_reaction", Description: "Add a reaction to an issue comment",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "comment_id": "Comment ID", "content": "Reaction: +1, -1, laugh, confused, heart, hooray, rocket, eyes"},
		Required:   []string{"owner", "repo", "comment_id", "content"},
	},
	{
		Name: "github_list_issue_reactions", Description: "List reactions on an issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},

	// ── Pull Requests ─────────────────────────────────────────────────
	{
		Name: "github_list_pulls", Description: "List pull requests for a repository. Start here for PR workflows when you know the repo. For cross-repo search, use search_issues with type:pr.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open, closed, all", "head": "Filter by head user:branch", "base": "Filter by base branch", "sort": "Sort: created, updated, popularity, long-running", "direction": "Direction: asc, desc", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_pull", Description: "Get a single pull request with full details. Use after list_pulls to drill into a specific PR. For the diff, follow up with get_pull_diff.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_get_pull_diff", Description: "Get the raw unified diff of a pull request. Use after get_pull for the full code diff.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_create_pull", Description: "Create a pull request. Requires head branch and base branch.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "title": "PR title", "head": "Head branch (or user:branch for cross-repo)", "base": "Base branch", "body": "PR body (markdown)", "draft": "Create as draft (true/false)"},
		Required:   []string{"owner", "repo", "title", "head", "base"},
	},
	{
		Name: "github_update_pull", Description: "Update a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "title": "New title", "body": "New body", "state": "State: open, closed", "base": "New base branch"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_list_pull_commits", Description: "List commits on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_list_pull_files", Description: "List files changed in a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_list_pull_reviews", Description: "List reviews on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_create_pull_review", Description: "Create a review on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "body": "Review body", "event": "Review action: APPROVE, REQUEST_CHANGES, COMMENT"},
		Required:   []string{"owner", "repo", "pull_number", "event"},
	},
	{
		Name: "github_list_pull_comments", Description: "List review comments on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_create_pull_comment", Description: "Create a review comment on a pull request diff",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "body": "Comment body", "commit_id": "SHA of the commit to comment on", "path": "Relative file path", "line": "Line number in the diff"},
		Required:   []string{"owner", "repo", "pull_number", "body", "commit_id", "path"},
	},
	{
		Name: "github_merge_pull", Description: "Merge a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "commit_message": "Merge commit message", "merge_method": "Method: merge, squash, rebase"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_list_requested_reviewers", Description: "List requested reviewers on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_request_reviewers", Description: "Request reviewers on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "reviewers": "Comma-separated usernames", "team_reviewers": "Comma-separated team slugs"},
		Required:   []string{"owner", "repo", "pull_number"},
	},

	// ── Pull Requests (extended) ──────────────────────────────────────
	{
		Name: "github_dismiss_pull_review", Description: "Dismiss a pull request review",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "review_id": "Review ID", "message": "Dismissal message"},
		Required:   []string{"owner", "repo", "pull_number", "review_id", "message"},
	},
	{
		Name: "github_update_pull_branch", Description: "Update a PR branch with the latest changes from the base branch",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "expected_head_sha": "Expected SHA of the PR head (for optimistic locking)"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_remove_reviewers", Description: "Remove requested reviewers from a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "pull_number": "Pull request number", "reviewers": "Comma-separated usernames", "team_reviewers": "Comma-separated team slugs"},
		Required:   []string{"owner", "repo", "pull_number"},
	},
	{
		Name: "github_list_pulls_with_commit", Description: "List pull requests that contain a specific commit",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Commit SHA", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "sha"},
	},

	// ── Git (low-level) ───────────────────────────────────────────────
	{
		Name: "github_get_commit", Description: "Get a commit by SHA including files changed. For comparing two refs, use compare_commits instead.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Commit SHA"},
		Required:   []string{"owner", "repo", "sha"},
	},
	{
		Name: "github_list_commits", Description: "List commits on a branch",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Branch name or SHA to start listing from", "path": "Only commits containing this file path", "author": "GitHub login or email to filter by", "since": "Only commits after this date (ISO 8601)", "until": "Only commits before this date (ISO 8601)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_ref", Description: "Get a git reference (branch or tag)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Reference (e.g., heads/main, tags/v1.0)"},
		Required:   []string{"owner", "repo", "ref"},
	},
	{
		Name: "github_create_ref", Description: "Create a git reference (branch or tag)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Full reference path (e.g., refs/heads/new-branch)", "sha": "SHA to point the ref at"},
		Required:   []string{"owner", "repo", "ref", "sha"},
	},
	{
		Name: "github_delete_ref", Description: "Delete a git reference",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Reference to delete (e.g., heads/old-branch)"},
		Required:   []string{"owner", "repo", "ref"},
	},
	{
		Name: "github_get_tree", Description: "Get a git tree (directory listing)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Tree SHA or branch name", "recursive": "Recurse into subtrees (true/false)"},
		Required:   []string{"owner", "repo", "sha"},
	},
	{
		Name: "github_create_tag", Description: "Create an annotated tag object",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "tag": "Tag name", "message": "Tag message", "sha": "SHA of object to tag", "type": "Object type: commit, tree, blob"},
		Required:   []string{"owner", "repo", "tag", "message", "sha"},
	},

	// ── Users ─────────────────────────────────────────────────────────
	{
		Name: "github_get_authenticated_user", Description: "Get the currently authenticated user",
		Parameters: map[string]string{},
	},
	{
		Name: "github_get_user", Description: "Get a user by username",
		Parameters: map[string]string{"username": "GitHub username"},
		Required:   []string{"username"},
	},
	{
		Name: "github_list_user_followers", Description: "List followers of a user",
		Parameters: map[string]string{"username": "GitHub username", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"username"},
	},
	{
		Name: "github_list_user_following", Description: "List users that a user follows",
		Parameters: map[string]string{"username": "GitHub username", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"username"},
	},
	{
		Name: "github_list_user_keys", Description: "List public SSH keys for a user",
		Parameters: map[string]string{"username": "GitHub username", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"username"},
	},

	// ── Organizations ─────────────────────────────────────────────────
	{
		Name: "github_get_org", Description: "Get an organization",
		Parameters: map[string]string{"org": "Organization login name"},
		Required:   []string{"org"},
	},
	{
		Name: "github_list_user_orgs", Description: "List organizations for a user",
		Parameters: map[string]string{"username": "GitHub username (empty for authenticated user)", "page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "github_list_org_members", Description: "List organization members",
		Parameters: map[string]string{"org": "Organization name", "role": "Filter: all, admin, member", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},
	{
		Name: "github_list_org_teams", Description: "List teams in an organization",
		Parameters: map[string]string{"org": "Organization name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},
	{
		Name: "github_get_team_by_slug", Description: "Get a team by slug",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug"},
		Required:   []string{"org", "slug"},
	},
	{
		Name: "github_list_team_members", Description: "List members of a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug", "role": "Filter: all, member, maintainer", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org", "slug"},
	},
	{
		Name: "github_list_team_repos", Description: "List repositories for a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org", "slug"},
	},

	// ── Teams (extended) ──────────────────────────────────────────────
	{
		Name: "github_create_team", Description: "Create a team in an organization",
		Parameters: map[string]string{"org": "Organization name", "name": "Team name", "description": "Team description", "privacy": "Privacy: secret, closed", "permission": "Default permission: pull, push"},
		Required:   []string{"org", "name"},
	},
	{
		Name: "github_edit_team", Description: "Update a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug", "name": "New team name", "description": "New description", "privacy": "Privacy: secret, closed", "permission": "Default permission: pull, push"},
		Required:   []string{"org", "slug", "name"},
	},
	{
		Name: "github_delete_team", Description: "Delete a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug"},
		Required:   []string{"org", "slug"},
	},
	{
		Name: "github_add_team_member", Description: "Add or update a user's membership in a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug", "username": "GitHub username", "role": "Role: member, maintainer"},
		Required:   []string{"org", "slug", "username"},
	},
	{
		Name: "github_remove_team_member", Description: "Remove a user from a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug", "username": "GitHub username"},
		Required:   []string{"org", "slug", "username"},
	},
	{
		Name: "github_add_team_repo", Description: "Add a repository to a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug", "owner": "Repository owner", "repo": "Repository name", "permission": "Permission: pull, triage, push, maintain, admin"},
		Required:   []string{"org", "slug", "owner", "repo"},
	},
	{
		Name: "github_remove_team_repo", Description: "Remove a repository from a team",
		Parameters: map[string]string{"org": "Organization name", "slug": "Team slug", "owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"org", "slug", "owner", "repo"},
	},
	{
		Name: "github_list_pending_org_invitations", Description: "List pending organization invitations",
		Parameters: map[string]string{"org": "Organization name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},
	{
		Name: "github_list_outside_collaborators", Description: "List outside collaborators for an organization",
		Parameters: map[string]string{"org": "Organization name", "filter": "Filter: 2fa_disabled, all", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},

	// ── Actions (CI/CD) ───────────────────────────────────────────────
	{
		Name: "github_list_workflows", Description: "List workflows in a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_workflow_runs", Description: "List workflow runs for a repository. Use to check CI status or recent builds.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "workflow_id": "Workflow ID or filename (e.g., ci.yml)", "branch": "Filter by branch", "event": "Filter by event (push, pull_request, etc.)", "status": "Filter: completed, action_required, cancelled, failure, neutral, skipped, stale, success, timed_out, in_progress, queued, requested, waiting, pending", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_workflow_run", Description: "Get a specific workflow run. Use after list_workflow_runs for full run details.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "run_id": "Workflow run ID"},
		Required:   []string{"owner", "repo", "run_id"},
	},
	{
		Name: "github_list_workflow_jobs", Description: "List jobs for a workflow run",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "run_id": "Workflow run ID", "filter": "Filter: latest, all", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "run_id"},
	},
	{
		Name: "github_download_workflow_logs", Description: "Get a URL to download workflow run logs",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "run_id": "Workflow run ID"},
		Required:   []string{"owner", "repo", "run_id"},
	},
	{
		Name: "github_rerun_workflow", Description: "Re-run a workflow run",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "run_id": "Workflow run ID"},
		Required:   []string{"owner", "repo", "run_id"},
	},
	{
		Name: "github_cancel_workflow_run", Description: "Cancel a workflow run",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "run_id": "Workflow run ID"},
		Required:   []string{"owner", "repo", "run_id"},
	},
	{
		Name: "github_list_repo_secrets", Description: "List repository Actions secrets (names only, not values)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_artifacts", Description: "List artifacts for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_environment_secrets", Description: "List secrets for an environment (names only)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "environment": "Environment name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "environment"},
	},
	{
		Name: "github_list_org_secrets", Description: "List organization Actions secrets (names only)",
		Parameters: map[string]string{"org": "Organization name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},

	// ── Actions (extended) ────────────────────────────────────────────
	{
		Name: "github_trigger_workflow", Description: "Trigger a workflow dispatch event",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "workflow_id": "Workflow filename (e.g., ci.yml)", "ref": "Git ref to run workflow on (branch or tag)"},
		Required:   []string{"owner", "repo", "workflow_id", "ref"},
	},
	{
		Name: "github_rerun_failed_jobs", Description: "Re-run only the failed jobs of a workflow run",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "run_id": "Workflow run ID"},
		Required:   []string{"owner", "repo", "run_id"},
	},
	{
		Name: "github_get_workflow_job", Description: "Get a single workflow job by ID",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "job_id": "Job ID"},
		Required:   []string{"owner", "repo", "job_id"},
	},
	{
		Name: "github_get_workflow_job_logs", Description: "Get a URL to download a single job's logs",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "job_id": "Job ID"},
		Required:   []string{"owner", "repo", "job_id"},
	},
	{
		Name: "github_delete_workflow_run", Description: "Delete a workflow run",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "run_id": "Workflow run ID"},
		Required:   []string{"owner", "repo", "run_id"},
	},
	{
		Name: "github_list_repo_variables", Description: "List repository Actions variables (names and values)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_create_repo_variable", Description: "Create a repository Actions variable",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "name": "Variable name", "value": "Variable value"},
		Required:   []string{"owner", "repo", "name", "value"},
	},
	{
		Name: "github_update_repo_variable", Description: "Update a repository Actions variable",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "name": "Variable name", "value": "New variable value"},
		Required:   []string{"owner", "repo", "name", "value"},
	},
	{
		Name: "github_delete_repo_variable", Description: "Delete a repository Actions variable",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "name": "Variable name"},
		Required:   []string{"owner", "repo", "name"},
	},
	{
		Name: "github_list_org_variables", Description: "List organization Actions variables (names and values)",
		Parameters: map[string]string{"org": "Organization name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},
	{
		Name: "github_list_env_variables", Description: "List environment Actions variables (names and values)",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "environment": "Environment name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "environment"},
	},
	{
		Name: "github_list_runners", Description: "List self-hosted runners for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_org_runners", Description: "List self-hosted runners for an organization",
		Parameters: map[string]string{"org": "Organization name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},

	// ── Checks ────────────────────────────────────────────────────────
	{
		Name: "github_list_check_runs", Description: "List check runs for a git reference",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Git ref (SHA, branch, or tag)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "ref"},
	},
	{
		Name: "github_get_check_run", Description: "Get a check run by ID",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "check_run_id": "Check run ID"},
		Required:   []string{"owner", "repo", "check_run_id"},
	},
	{
		Name: "github_list_check_suites", Description: "List check suites for a git reference",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Git ref (SHA, branch, or tag)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "ref"},
	},

	// ── Releases ──────────────────────────────────────────────────────
	{
		Name: "github_list_releases", Description: "List releases for a repository. Start here for release history, versioning, and what shipped.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_release", Description: "Get a release by ID",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "release_id": "Release ID"},
		Required:   []string{"owner", "repo", "release_id"},
	},
	{
		Name: "github_get_latest_release", Description: "Get the latest release for a repository. Use to find what version is current or what shipped most recently.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_create_release", Description: "Create a release",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "tag_name": "Tag name for the release", "name": "Release name", "body": "Release notes (markdown)", "draft": "Create as draft (true/false)", "prerelease": "Mark as pre-release (true/false)", "target_commitish": "Branch or SHA to tag (defaults to default branch)"},
		Required:   []string{"owner", "repo", "tag_name"},
	},
	{
		Name: "github_delete_release", Description: "Delete a release",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "release_id": "Release ID"},
		Required:   []string{"owner", "repo", "release_id"},
	},
	{
		Name: "github_list_release_assets", Description: "List assets for a release",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "release_id": "Release ID", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "release_id"},
	},

	// ── Gists ─────────────────────────────────────────────────────────
	{
		Name: "github_list_gists", Description: "List gists for the authenticated user or a specific user",
		Parameters: map[string]string{"username": "GitHub username (empty for authenticated user)", "page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "github_get_gist", Description: "Get a gist by ID",
		Parameters: map[string]string{"id": "Gist ID"},
		Required:   []string{"id"},
	},
	{
		Name: "github_create_gist", Description: "Create a gist",
		Parameters: map[string]string{"description": "Gist description", "public": "Public gist (true/false)", "filename": "Filename for the gist content", "content": "File content"},
		Required:   []string{"filename", "content"},
	},

	// ── Search ────────────────────────────────────────────────────────
	{
		Name: "github_search_topics", Description: "Search GitHub topics",
		Parameters: map[string]string{"query": "Search query", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},
	{
		Name: "github_search_labels", Description: "Search labels in a repository",
		Parameters: map[string]string{"repository_id": "Repository numeric ID", "query": "Search query", "sort": "Sort: created, updated", "order": "Order: asc, desc", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"repository_id", "query"},
	},
	{
		Name: "github_search_code", Description: "Search code across GitHub repositories. Start here to find files, functions, or strings across repos.",
		Parameters: map[string]string{"query": "Search query (supports qualifiers like language:go, repo:owner/name)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},
	{
		Name: "github_search_issues", Description: "Search issues and pull requests across GitHub. Start here for cross-repo issue/PR discovery. Add is:pr or is:issue to filter. Use for cross-repo bug, ticket, or PR discovery.",
		Parameters: map[string]string{"query": "Search query (supports qualifiers like is:issue, is:pr, repo:owner/name, state:open)", "sort": "Sort: comments, reactions, created, updated", "order": "Order: asc, desc", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},
	{
		Name: "github_search_users", Description: "Search GitHub users",
		Parameters: map[string]string{"query": "Search query (supports qualifiers like location:, language:, followers:>N)", "sort": "Sort: followers, repositories, joined", "order": "Order: asc, desc", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},
	{
		Name: "github_search_commits", Description: "Search commits across GitHub",
		Parameters: map[string]string{"query": "Search query (supports qualifiers like author:, repo:, committer:)", "sort": "Sort: author-date, committer-date", "order": "Order: asc, desc", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},

	// ── Activity ──────────────────────────────────────────────────────
	{
		Name: "github_list_stargazers", Description: "List stargazers for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_watchers", Description: "List watchers (subscribers) of a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_notifications", Description: "List notifications for the authenticated user",
		Parameters: map[string]string{"all": "Show all notifications including read (true/false)", "participating": "Only show participating notifications (true/false)", "page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "github_list_repo_events", Description: "List events for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},

	// ── Activity (extended) ───────────────────────────────────────────
	{
		Name: "github_mark_notifications_read", Description: "Mark all notifications as read",
		Parameters: map[string]string{},
	},
	{
		Name: "github_star_repo", Description: "Star a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_unstar_repo", Description: "Unstar a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_starred", Description: "List repositories starred by a user",
		Parameters: map[string]string{"username": "GitHub username (empty for authenticated user)", "sort": "Sort: created, updated", "direction": "Direction: asc, desc", "page": "Page number", "per_page": "Results per page"},
	},

	// ── Code Scanning ─────────────────────────────────────────────────
	{
		Name: "github_list_code_scanning_alerts", Description: "List code scanning (SAST) alerts for a repository. Start here for security vulnerabilities found by CodeQL or other analyzers.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open, closed, dismissed, fixed", "ref": "Git ref to filter by", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_code_scanning_alert", Description: "Get a code scanning alert",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "alert_number": "Alert number"},
		Required:   []string{"owner", "repo", "alert_number"},
	},

	// ── Secret Scanning ───────────────────────────────────────────────
	{
		Name: "github_list_secret_scanning_alerts", Description: "List secret scanning alerts for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open, resolved", "secret_type": "Filter by secret type", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},

	// ── Secret Scanning (extended) ────────────────────────────────────
	{
		Name: "github_get_secret_scanning_alert", Description: "Get a single secret scanning alert",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "alert_number": "Alert number"},
		Required:   []string{"owner", "repo", "alert_number"},
	},

	// ── Dependabot ────────────────────────────────────────────────────
	{
		Name: "github_list_dependabot_alerts", Description: "List Dependabot dependency vulnerability alerts for a repository. Start here for CVE impact on dependencies.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: auto_dismissed, dismissed, fixed, open", "severity": "Severity: low, medium, high, critical", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},

	// ── Dependabot (extended) ─────────────────────────────────────────
	{
		Name: "github_get_dependabot_alert", Description: "Get a single Dependabot alert",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "alert_number": "Alert number"},
		Required:   []string{"owner", "repo", "alert_number"},
	},

	// ── Code Scanning (extended) ─────────────────────────────────────
	{
		Name: "github_list_code_scanning_analyses", Description: "List code scanning SARIF analyses for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "ref": "Git ref to filter by", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},

	// ── SBOM ─────────────────────────────────────────────────────────
	{
		Name: "github_get_sbom", Description: "Get the software bill of materials (SBOM) for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},

	// ── Copilot ───────────────────────────────────────────────────────
	{
		Name: "github_get_copilot_org_usage", Description: "Get Copilot usage metrics for an organization",
		Parameters: map[string]string{"org": "Organization name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"org"},
	},

	// ── Webhooks ──────────────────────────────────────────────────────
	{
		Name: "github_list_hooks", Description: "List webhooks for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_create_hook", Description: "Create a webhook for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "url": "Webhook payload URL", "content_type": "Content type: json, form", "events": "Comma-separated events (push, pull_request, issues, etc.)", "active": "Active (true/false, default true)"},
		Required:   []string{"owner", "repo", "url"},
	},
	{
		Name: "github_delete_hook", Description: "Delete a webhook",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "hook_id": "Webhook ID"},
		Required:   []string{"owner", "repo", "hook_id"},
	},

	// ── Deploy Keys ───────────────────────────────────────────────────
	{
		Name: "github_list_deploy_keys", Description: "List deploy keys for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},

	// ── Rate Limit ────────────────────────────────────────────────────
	{
		Name: "github_get_rate_limit", Description: "Get API rate limit status for the authenticated user",
		Parameters: map[string]string{},
	},
}
