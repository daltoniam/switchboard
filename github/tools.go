package github

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Repositories ──────────────────────────────────────────────────
	{
		Name: "github_search_repos", Description: "Search GitHub repositories",
		Parameters: map[string]string{"query": "Search query", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},
	{
		Name: "github_get_repo", Description: "Get a repository by owner/name",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_user_repos", Description: "List repositories for a user",
		Parameters: map[string]string{"username": "GitHub username", "type": "Type: all, owner, member (default: owner)", "sort": "Sort: created, updated, pushed, full_name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"username"},
	},
	{
		Name: "github_list_org_repos", Description: "List repositories for an organization",
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
		Name: "github_compare_commits", Description: "Compare two commits/branches/tags",
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

	// ── Issues ────────────────────────────────────────────────────────
	{
		Name: "github_list_issues", Description: "List issues for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open, closed, all", "labels": "Comma-separated label names", "sort": "Sort: created, updated, comments", "direction": "Direction: asc, desc", "assignee": "Filter by assignee username", "milestone": "Milestone number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_issue", Description: "Get a single issue",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Issue number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_create_issue", Description: "Create an issue",
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

	// ── Pull Requests ─────────────────────────────────────────────────
	{
		Name: "github_list_prs", Description: "List pull requests for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open, closed, all", "head": "Filter by head user:branch", "base": "Filter by base branch", "sort": "Sort: created, updated, popularity, long-running", "direction": "Direction: asc, desc", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_pr", Description: "Get a single pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_create_pr", Description: "Create a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "title": "PR title", "head": "Head branch (or user:branch for cross-repo)", "base": "Base branch", "body": "PR body (markdown)", "draft": "Create as draft (true/false)"},
		Required:   []string{"owner", "repo", "title", "head", "base"},
	},
	{
		Name: "github_update_pr", Description: "Update a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "title": "New title", "body": "New body", "state": "State: open, closed", "base": "New base branch"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_pr_commits", Description: "List commits on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_pr_files", Description: "List files changed in a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_pr_reviews", Description: "List reviews on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_create_pr_review", Description: "Create a review on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "body": "Review body", "event": "Review action: APPROVE, REQUEST_CHANGES, COMMENT"},
		Required:   []string{"owner", "repo", "number", "event"},
	},
	{
		Name: "github_list_pr_comments", Description: "List review comments on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_create_pr_comment", Description: "Create a review comment on a pull request diff",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "body": "Comment body", "commit_id": "SHA of the commit to comment on", "path": "Relative file path", "line": "Line number in the diff"},
		Required:   []string{"owner", "repo", "number", "body", "commit_id", "path"},
	},
	{
		Name: "github_merge_pr", Description: "Merge a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "commit_message": "Merge commit message", "merge_method": "Method: merge, squash, rebase"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_list_requested_reviewers", Description: "List requested reviewers on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "github_request_reviewers", Description: "Request reviewers on a pull request",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "PR number", "reviewers": "Comma-separated usernames", "team_reviewers": "Comma-separated team slugs"},
		Required:   []string{"owner", "repo", "number"},
	},

	// ── Git (low-level) ───────────────────────────────────────────────
	{
		Name: "github_get_commit", Description: "Get a commit by SHA",
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

	// ── Actions (CI/CD) ───────────────────────────────────────────────
	{
		Name: "github_list_workflows", Description: "List workflows in a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_list_workflow_runs", Description: "List workflow runs",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "workflow_id": "Workflow ID or filename (e.g., ci.yml)", "branch": "Filter by branch", "event": "Filter by event (push, pull_request, etc.)", "status": "Filter: completed, action_required, cancelled, failure, neutral, skipped, stale, success, timed_out, in_progress, queued, requested, waiting, pending", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_workflow_run", Description: "Get a specific workflow run",
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
		Name: "github_list_releases", Description: "List releases for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "github_get_release", Description: "Get a release by ID",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "release_id": "Release ID"},
		Required:   []string{"owner", "repo", "release_id"},
	},
	{
		Name: "github_get_latest_release", Description: "Get the latest release",
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
		Name: "github_search_code", Description: "Search code across GitHub repositories",
		Parameters: map[string]string{"query": "Search query (supports qualifiers like language:go, repo:owner/name)", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"query"},
	},
	{
		Name: "github_search_issues", Description: "Search issues and pull requests across GitHub",
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

	// ── Code Scanning ─────────────────────────────────────────────────
	{
		Name: "github_list_code_scanning_alerts", Description: "List code scanning alerts for a repository",
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

	// ── Dependabot ────────────────────────────────────────────────────
	{
		Name: "github_list_dependabot_alerts", Description: "List Dependabot alerts for a repository",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: auto_dismissed, dismissed, fixed, open", "severity": "Severity: low, medium, high, critical", "page": "Page number", "per_page": "Results per page"},
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
