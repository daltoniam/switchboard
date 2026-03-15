package jira

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Issues ───────────────────────────────────────────────────────
	{
		Name: "jira_search_issues", Description: "Search issues using JQL (Jira Query Language). Start here for most issue workflows. Returns paginated results",
		Parameters: map[string]string{"jql": "JQL query (e.g., 'project = PROJ AND status = Open')", "fields": "Comma-separated fields to return (default: summary,status,assignee,priority,issuetype)", "start_at": "Pagination offset (0-based)", "max_results": "Max results per page (default 50, max 100)"},
		Required:   []string{"jql"},
	},
	{
		Name: "jira_get_issue", Description: "Get full details of a specific issue by key or ID",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123) or ID", "fields": "Comma-separated fields to return (default: all)", "expand": "Comma-separated expansions (e.g., changelog,renderedFields)"},
		Required:   []string{"issue_key"},
	},
	{
		Name: "jira_create_issue", Description: "Create a new issue. Use jira_list_issue_types to find valid issue type names for the project",
		Parameters: map[string]string{"project_key": "Project key (e.g., PROJ)", "issue_type": "Issue type name (e.g., Bug, Task, Story)", "summary": "Issue summary/title", "description": "Issue description (plain text, converted to ADF)", "priority": "Priority name (e.g., High, Medium, Low)", "assignee_id": "Account ID of assignee (use jira_search_users to find)", "labels": "Comma-separated labels", "parent_key": "Parent issue key for subtasks (e.g., PROJ-100)"},
		Required:   []string{"project_key", "issue_type", "summary"},
	},
	{
		Name: "jira_update_issue", Description: "Update an existing issue's fields",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "summary": "New summary", "description": "New description (plain text, converted to ADF)", "priority": "Priority name", "assignee_id": "Account ID of assignee (empty string to unassign)", "labels": "Comma-separated labels (replaces existing)"},
		Required:   []string{"issue_key"},
	},
	{
		Name: "jira_delete_issue", Description: "Delete an issue",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "delete_subtasks": "Also delete subtasks (true/false, default false)"},
		Required:   []string{"issue_key"},
	},
	{
		Name: "jira_transition_issue", Description: "Transition an issue to a new status. Use jira_get_transitions first to find valid transition IDs",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "transition_id": "Transition ID (use jira_get_transitions to find valid IDs)"},
		Required:   []string{"issue_key", "transition_id"},
	},
	{
		Name: "jira_get_transitions", Description: "List available transitions for an issue. Use before jira_transition_issue to find valid transition IDs",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)"},
		Required:   []string{"issue_key"},
	},
	{
		Name: "jira_assign_issue", Description: "Assign an issue to a user",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "account_id": "Account ID of assignee (use jira_search_users to find, or empty/-1 to unassign)"},
		Required:   []string{"issue_key", "account_id"},
	},
	{
		Name: "jira_list_comments", Description: "List comments on an issue",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "start_at": "Pagination offset", "max_results": "Max results per page"},
		Required:   []string{"issue_key"},
	},
	{
		Name: "jira_add_comment", Description: "Add a comment to an issue",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "body": "Comment body (plain text, converted to ADF)"},
		Required:   []string{"issue_key", "body"},
	},
	{
		Name: "jira_update_comment", Description: "Update an existing comment",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "comment_id": "Comment ID", "body": "New comment body (plain text, converted to ADF)"},
		Required:   []string{"issue_key", "comment_id", "body"},
	},
	{
		Name: "jira_delete_comment", Description: "Delete a comment from an issue",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "comment_id": "Comment ID"},
		Required:   []string{"issue_key", "comment_id"},
	},
	{
		Name: "jira_list_issue_links", Description: "List links on an issue (blocks, is blocked by, duplicates, etc.)",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)"},
		Required:   []string{"issue_key"},
	},
	{
		Name: "jira_create_issue_link", Description: "Create a link between two issues",
		Parameters: map[string]string{"type_name": "Link type name (e.g., Blocks, Duplicate, Cloners)", "inward_issue": "Issue key for the inward side", "outward_issue": "Issue key for the outward side"},
		Required:   []string{"type_name", "inward_issue", "outward_issue"},
	},
	{
		Name: "jira_delete_issue_link", Description: "Delete an issue link by link ID",
		Parameters: map[string]string{"link_id": "Issue link ID"},
		Required:   []string{"link_id"},
	},

	// ── Projects ─────────────────────────────────────────────────────
	{
		Name: "jira_list_projects", Description: "List all accessible projects",
		Parameters: map[string]string{"start_at": "Pagination offset", "max_results": "Max results per page (default 50)", "query": "Filter projects by name"},
	},
	{
		Name: "jira_get_project", Description: "Get details of a specific project",
		Parameters: map[string]string{"project_key": "Project key or ID"},
		Required:   []string{"project_key"},
	},
	{
		Name: "jira_list_project_components", Description: "List components in a project",
		Parameters: map[string]string{"project_key": "Project key or ID"},
		Required:   []string{"project_key"},
	},
	{
		Name: "jira_list_project_versions", Description: "List versions (releases) in a project",
		Parameters: map[string]string{"project_key": "Project key or ID"},
		Required:   []string{"project_key"},
	},
	{
		Name: "jira_list_project_statuses", Description: "List valid statuses for a project's issue types",
		Parameters: map[string]string{"project_key": "Project key or ID"},
		Required:   []string{"project_key"},
	},

	// ── Boards & Sprints (Agile API) ────────────────────────────────
	{
		Name: "jira_list_boards", Description: "List all agile boards",
		Parameters: map[string]string{"start_at": "Pagination offset", "max_results": "Max results per page (default 50)", "project_key": "Filter by project key", "type": "Board type: scrum, kanban, simple"},
	},
	{
		Name: "jira_get_board", Description: "Get details of an agile board",
		Parameters: map[string]string{"board_id": "Board ID"},
		Required:   []string{"board_id"},
	},
	{
		Name: "jira_list_sprints", Description: "List sprints for a board",
		Parameters: map[string]string{"board_id": "Board ID", "start_at": "Pagination offset", "max_results": "Max results per page", "state": "Filter by state: active, future, closed"},
		Required:   []string{"board_id"},
	},
	{
		Name: "jira_get_sprint", Description: "Get details of a sprint",
		Parameters: map[string]string{"sprint_id": "Sprint ID"},
		Required:   []string{"sprint_id"},
	},
	{
		Name: "jira_create_sprint", Description: "Create a new sprint",
		Parameters: map[string]string{"board_id": "Board ID (origin board)", "name": "Sprint name", "start_date": "Start date (ISO 8601)", "end_date": "End date (ISO 8601)", "goal": "Sprint goal"},
		Required:   []string{"board_id", "name"},
	},
	{
		Name: "jira_update_sprint", Description: "Update a sprint's details",
		Parameters: map[string]string{"sprint_id": "Sprint ID", "name": "Sprint name", "state": "Sprint state: active, future, closed", "start_date": "Start date (ISO 8601)", "end_date": "End date (ISO 8601)", "goal": "Sprint goal"},
		Required:   []string{"sprint_id"},
	},
	{
		Name: "jira_get_sprint_issues", Description: "List issues in a sprint",
		Parameters: map[string]string{"sprint_id": "Sprint ID", "start_at": "Pagination offset", "max_results": "Max results per page", "jql": "Additional JQL filter"},
		Required:   []string{"sprint_id"},
	},
	{
		Name: "jira_move_issues_to_sprint", Description: "Move issues to a sprint",
		Parameters: map[string]string{"sprint_id": "Sprint ID", "issues": "Comma-separated issue keys (e.g., PROJ-1,PROJ-2)"},
		Required:   []string{"sprint_id", "issues"},
	},
	{
		Name: "jira_list_board_backlog", Description: "List issues in the backlog of a board",
		Parameters: map[string]string{"board_id": "Board ID", "start_at": "Pagination offset", "max_results": "Max results per page", "jql": "Additional JQL filter"},
		Required:   []string{"board_id"},
	},
	{
		Name: "jira_get_board_config", Description: "Get configuration of an agile board (columns, estimation, ranking)",
		Parameters: map[string]string{"board_id": "Board ID"},
		Required:   []string{"board_id"},
	},

	// ── Users ────────────────────────────────────────────────────────
	{
		Name:       "jira_get_myself", Description: "Get the current authenticated user's details",
		Parameters: map[string]string{},
	},
	{
		Name: "jira_search_users", Description: "Search for users by name or email. Use to find account IDs for assignment",
		Parameters: map[string]string{"query": "Search query (name or email)"},
		Required:   []string{"query"},
	},
	{
		Name: "jira_get_user", Description: "Get details of a specific user by account ID",
		Parameters: map[string]string{"account_id": "User account ID"},
		Required:   []string{"account_id"},
	},

	// ── Metadata ─────────────────────────────────────────────────────
	{
		Name: "jira_list_issue_types", Description: "List all issue types available in the Jira instance",
		Parameters: map[string]string{},
	},
	{
		Name: "jira_list_priorities", Description: "List all issue priorities",
		Parameters: map[string]string{},
	},
	{
		Name: "jira_list_statuses", Description: "List all issue statuses",
		Parameters: map[string]string{},
	},
	{
		Name: "jira_list_labels", Description: "List all labels used across the instance",
		Parameters: map[string]string{},
	},
	{
		Name: "jira_list_fields", Description: "List all fields (system and custom) available in the instance",
		Parameters: map[string]string{},
	},
	{
		Name: "jira_list_filters", Description: "Search saved filters",
		Parameters: map[string]string{"filter_name": "Filter by name (substring match)", "start_at": "Pagination offset", "max_results": "Max results per page"},
	},
	{
		Name: "jira_get_filter", Description: "Get details of a saved filter including its JQL",
		Parameters: map[string]string{"filter_id": "Filter ID"},
		Required:   []string{"filter_id"},
	},

	// ── Worklogs & Info ──────────────────────────────────────────────
	{
		Name: "jira_list_worklogs", Description: "List work logs for an issue",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "start_at": "Pagination offset", "max_results": "Max results per page"},
		Required:   []string{"issue_key"},
	},
	{
		Name: "jira_add_worklog", Description: "Add a work log entry to an issue",
		Parameters: map[string]string{"issue_key": "Issue key (e.g., PROJ-123)", "time_spent": "Time spent (e.g., 2h, 30m, 1d)", "comment": "Work log comment (plain text)", "started": "When work started (ISO 8601, defaults to now)"},
		Required:   []string{"issue_key", "time_spent"},
	},
	{
		Name:       "jira_get_server_info", Description: "Get Jira instance server info (version, deployment type, URLs)",
		Parameters: map[string]string{},
	},
}
