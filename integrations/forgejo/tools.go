package forgejo

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "forgejo_list_user_repos", Description: "List Forgejo repositories accessible to the authenticated user. Start here to discover repository owner/repo names, including private repos; supply username for a specific user's repositories.",
		Parameters: map[string]string{"username": "Optional username; omit for authenticated user's accessible repositories", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
	},
	{
		Name: "forgejo_search_repos", Description: "Search Forgejo repositories by keyword. Start here to find a repository by name; use list_user_repos for accessible private repositories.",
		Parameters: map[string]string{"query": "Repository search keyword (not GitHub search syntax)", "sort": "Sort: alpha, created, updated, size, id", "order": "Order: asc, desc", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"query"},
	},
	{
		Name: "forgejo_get_repo", Description: "Get Forgejo repository metadata, permissions, and default branch. Use after list_user_repos or search_repos with owner/repo.",
		Parameters: map[string]string{"owner": "Repository owner username or organization", "repo": "Repository name"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_list_org_repos", Description: "List repositories belonging to a Forgejo organization. Use after list_user_orgs to discover owner/repo names.",
		Parameters: map[string]string{"org": "Organization name", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"org"},
	},
	{
		Name: "forgejo_list_user_orgs", Description: "List Forgejo organizations for the authenticated user or a username. Use before list_org_repos to discover organization repositories.",
		Parameters: map[string]string{"username": "Optional username; omit for authenticated user", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
	},
	{
		Name: "forgejo_get_current_user", Description: "Get the authenticated Forgejo user account and login. Use to verify token identity before repository or issue actions.",
		Parameters: map[string]string{},
	},
	{
		Name: "forgejo_list_branches", Description: "List Forgejo repository branches and commit references. Use after get_repo, before get_branch or create_pull.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_get_branch", Description: "Get a Forgejo branch and its latest commit. Use after list_branches; branch names may contain slashes.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "branch": "Literal branch name, including nested names such as feature/docs; do not URL-encode # or %"},
		Required:   []string{"owner", "repo", "branch"},
	},
	{
		Name: "forgejo_list_commits", Description: "List Forgejo repository commit history, optionally by branch or file path. Use before get_commit to investigate code changes. With nonempty path, Forgejo uses a server-controlled page size: omit per_page and use page to continue; results are not sliced locally.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Optional branch, tag, or commit SHA", "path": "Optional literal relative file/directory path; omit or empty for all paths, no dot traversal", "page": "Page number, positive integer (default 1); with path, advances server-sized pages", "per_page": "Results per page, integer 1-50 (default 30); rejected with nonempty path because page size is server-controlled"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_get_commit", Description: "Get a Forgejo commit's metadata and changed files. Use after list_commits with its SHA.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "sha": "Commit SHA or reference"},
		Required:   []string{"owner", "repo", "sha"},
	},
	{
		Name: "forgejo_get_file_contents", Description: "Read one Forgejo repository file, including its encoding and base64-encoded content. Use after list_directory; directories must use list_directory. Response budget is 1 MiB.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "path": "Literal relative file path; #, ?, % and nested slashes allowed, do not URL-encode; no leading slash or dot traversal", "ref": "Optional branch, tag, or commit SHA; omit for default branch"},
		Required:   []string{"owner", "repo", "path"},
	},
	{
		Name: "forgejo_list_directory", Description: "List files and subdirectories in a Forgejo repository directory. Use before get_file_contents to browse source code; this endpoint is not paginated.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "path": "Optional relative directory path; omit or empty for root, no dot traversal", "ref": "Optional branch, tag, or commit SHA"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_list_releases", Description: "List Forgejo repository releases and version tags. Use before get_release to find release_id and release notes.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "draft": "Optional boolean draft filter", "prerelease": "Optional boolean prerelease filter", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_get_release", Description: "Get Forgejo release notes and assets. Use release_id from list_releases, not a tag or issue number.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "release_id": "Positive integer release ID from list_releases"},
		Required:   []string{"owner", "repo", "release_id"},
	},
	{
		Name: "forgejo_list_labels", Description: "List Forgejo repository labels and numeric label IDs. Use before create_issue, create_pull, or update_pull to assign labels.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_list_issues", Description: "List Forgejo issues, bugs, tickets, and tasks, excluding pull requests. Start here for issue triage; returns repository-local number for follow-up tools.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open (default), closed, all", "labels": "Optional array of label names to filter issues", "query": "Optional issue search keyword", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_get_issue", Description: "Get a Forgejo issue's description, status, and assignees. Use after list_issues with repository-local number, never global id.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local issue number, not global ID"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_create_issue", Description: "Create a Forgejo bug, ticket, or task. Use after get_repo and list_labels; returns the local issue number for comments and updates.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "title": "Nonempty issue title", "body": "Optional Markdown description", "assignees": "Optional array of usernames", "labels": "Optional array of positive numeric label IDs from list_labels", "milestone": "Optional positive milestone ID", "closed": "Optional boolean; create as closed when true"},
		Required:   []string{"owner", "repo", "title"},
	},
	{
		Name: "forgejo_update_issue", Description: "Update a Forgejo issue's title, body, status, or assignees. Use after get_issue. Omitted fields stay unchanged; body empty string clears text, assignees [] clears assignments; null is rejected.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local issue number, not global ID", "title": "Optional nonempty title", "body": "Optional Markdown body; empty string clears", "state": "Optional state: open, closed", "assignees": "Optional array of usernames; [] clears"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_list_issue_comments", Description: "List Forgejo issue or pull request conversation comments. Use after get_issue or get_pull with its local number, not global id. Forgejo returns an unpaginated list; narrow large conversations with since and before timestamp filters, not page/per_page.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local issue or PR number", "since": "Optional nonzero whole-second RFC3339 timestamp with timezone (no fractional seconds); comments updated after this time", "before": "Optional nonzero whole-second RFC3339 timestamp with timezone (no fractional seconds); comments updated before this time, must be later than since"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_create_issue_comment", Description: "Post a Forgejo issue or pull request conversation comment. Use after get_issue or get_pull; for a formal code review use create_pull_review.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local issue or PR number", "body": "Nonempty Markdown comment"},
		Required:   []string{"owner", "repo", "number", "body"},
	},
	{
		Name: "forgejo_list_pulls", Description: "List Forgejo pull requests for code review and merge work. Start here to find PRs; use repository-local number for get_pull and review tools, never global id.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "state": "State: open (default), closed, all", "sort": "Sort: oldest, recentupdate, leastupdate, mostcomment, leastcomment, priority", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"owner", "repo"},
	},
	{
		Name: "forgejo_get_pull", Description: "Get a Forgejo pull request's description, branches, and merge status. Use after list_pulls with local number, then get_pull_diff for code review.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local PR number, not global ID"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_create_pull", Description: "Create a Forgejo pull request proposing code changes. Use after list_branches; head may be owner:branch for a fork, base is the target branch.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "title": "Nonempty pull request title", "head": "Source branch, optionally username:branch for a fork", "base": "Target branch", "body": "Optional Markdown description", "assignees": "Optional array of usernames", "labels": "Optional array of positive numeric label IDs from list_labels", "milestone": "Optional positive milestone ID"},
		Required:   []string{"owner", "repo", "title", "head", "base"},
	},
	{
		Name: "forgejo_update_pull", Description: "Update a Forgejo pull request's description, target branch, state, labels, or assignments. Use after get_pull. Omitted fields stay unchanged; empty body/arrays and false are preserved; null is rejected.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local PR number, not global ID", "title": "Optional nonempty title", "body": "Optional Markdown body; empty string clears", "base": "Optional new target branch", "state": "Optional state: open, closed", "assignees": "Optional username array; [] clears", "labels": "Optional positive numeric label ID array; [] clears", "allow_maintainer_edit": "Optional boolean permission to edit the source branch"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_get_pull_diff", Description: "Get a Forgejo pull request's unified code diff as {diff: string}, not base64. Use after get_pull for code review; response budget is 1 MiB, use list_pull_files to inspect changed paths.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local PR number, not global ID"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_list_pull_files", Description: "List changed files and additions/deletions for a Forgejo pull request. Use after get_pull to scope a code review before get_pull_diff.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local PR number, not global ID", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_list_pull_reviews", Description: "List Forgejo pull request reviews, approvals, and requested changes. Use after get_pull before creating a review or merging.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local PR number, not global ID", "page": "Page number, positive integer (default 1)", "per_page": "Results per page, integer 1-50 (default 30)"},
		Required:   []string{"owner", "repo", "number"},
	},
	{
		Name: "forgejo_create_pull_review", Description: "Submit a Forgejo pull request review: approve, comment, or request changes. Use after get_pull_diff and list_pull_reviews; inline file comments are not supported.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local PR number, not global ID", "event": "Review event: APPROVED, COMMENT, REQUEST_CHANGES", "body": "Markdown review; required and nonempty except for APPROVED", "commit_id": "Optional commit SHA being reviewed"},
		Required:   []string{"owner", "repo", "number", "event"},
	},
	{
		Name: "forgejo_merge_pull", Description: "Merge a Forgejo pull request after reviewing get_pull, get_pull_diff, and list_pull_reviews. Honors branch protections; no force merge. Supply sha to reject an unexpectedly changed head.",
		Parameters: map[string]string{"owner": "Repository owner", "repo": "Repository name", "number": "Positive repository-local PR number, not global ID", "merge_method": "Merge style: merge (default), rebase, rebase-merge, squash, fast-forward-only", "sha": "Optional expected head commit SHA", "commit_title": "Optional merge commit title", "commit_message": "Optional merge commit message", "delete_branch_after_merge": "Optional boolean; delete source branch after merge (default false)"},
		Required:   []string{"owner", "repo", "number"},
	},
}
