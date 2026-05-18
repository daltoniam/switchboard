package jira

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Issues ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("jira_search_issues"), Description: "Search issues using JQL (Jira Query Language). Start here for most issue workflows. Returns paginated results; pass nextPageToken from response to fetch next page",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("jql"), Description: "JQL query (e.g., 'project = PROJ AND status = Open')", Required: true}, {Name: mcp.ParamName("fields"), Description: "Comma-separated fields to return (default: summary,status,assignee,priority,issuetype)"}, {Name: mcp.ParamName("next_page_token"), Description: "Cursor for next page (from previous response's nextPageToken field)"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page (default 200, server may cap)"}},
	},
	{
		Name: mcp.ToolName("jira_get_issue"), Description: "Get full details of a specific issue by key or ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123) or ID", Required: true}, {Name: mcp.ParamName("fields"), Description: "Comma-separated fields to return (default: all)"}, {Name: mcp.ParamName("expand"), Description: "Comma-separated expansions (e.g., changelog,renderedFields)"}},
	},
	{
		Name: mcp.ToolName("jira_create_issue"), Description: "Create a new issue. Use jira_list_issue_types to find valid issue type names. Supports custom fields — use jira_list_fields to discover field IDs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_key"), Description: "Project key (e.g., PROJ)", Required: true}, {Name: mcp.ParamName("issue_type"), Description: "Issue type name (e.g., Bug, Task, Story)", Required: true}, {Name: mcp.ParamName("summary"), Description: "Issue summary/title", Required: true}, {Name: mcp.ParamName("description"), Description: "Issue description (plain text, converted to ADF)"}, {Name: mcp.ParamName("priority"), Description: "Priority name (e.g., High, Medium, Low)"}, {Name: mcp.ParamName("assignee_id"), Description: "Account ID of assignee (use jira_search_users to find)"}, {Name: mcp.ParamName("labels"), Description: "Comma-separated labels"}, {Name: mcp.ParamName("parent_key"), Description: "Parent issue key for subtasks (e.g., PROJ-100)"}, {Name: mcp.ParamName("custom_fields"), Description: `JSON object of custom field values keyed by field ID (e.g. {"customfield_10001": "value", "customfield_10002": {"id": "10100"}}). Use jira_list_fields to discover field IDs and types`}},
	},
	{
		Name: mcp.ToolName("jira_update_issue"), Description: "Update an existing issue's fields including custom fields — use jira_list_fields to discover custom field IDs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("summary"), Description: "New summary"}, {Name: mcp.ParamName("description"), Description: "New description (plain text, converted to ADF)"}, {Name: mcp.ParamName("priority"), Description: "Priority name"}, {Name: mcp.ParamName("assignee_id"), Description: "Account ID of assignee (empty string to unassign)"}, {Name: mcp.ParamName("labels"), Description: "Comma-separated labels (replaces existing)"}, {Name: mcp.ParamName("custom_fields"), Description: `JSON object of custom field values keyed by field ID (e.g. {"customfield_10001": "value", "customfield_10002": {"id": "10100"}}). Use jira_list_fields to discover field IDs and types`}},
	},
	{
		Name: mcp.ToolName("jira_delete_issue"), Description: "Delete an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("delete_subtasks"), Description: "Also delete subtasks (true/false, default false)"}},
	},
	{
		Name: mcp.ToolName("jira_transition_issue"), Description: "Transition an issue to a new status. Use jira_get_transitions first to find valid transition IDs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("transition_id"), Description: "Transition ID (use jira_get_transitions to find valid IDs)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_get_transitions"), Description: "List available transitions for an issue. Use before jira_transition_issue to find valid transition IDs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_assign_issue"), Description: "Assign an issue to a user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("account_id"), Description: "Account ID of assignee (use jira_search_users to find, or empty/-1 to unassign)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_list_comments"), Description: "List comments on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}},
	},
	{
		Name: mcp.ToolName("jira_add_comment"), Description: "Add a comment to an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("body"), Description: "Comment body (plain text, converted to ADF)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_update_comment"), Description: "Update an existing comment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}, {Name: mcp.ParamName("body"), Description: "New comment body (plain text, converted to ADF)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_delete_comment"), Description: "Delete a comment from an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_list_issue_links"), Description: "List links on an issue (blocks, is blocked by, duplicates, etc.)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_create_issue_link"), Description: "Create a link between two issues",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("type_name"), Description: "Link type name (e.g., Blocks, Duplicate, Cloners)", Required: true}, {Name: mcp.ParamName("inward_issue"), Description: "Issue key for the inward side", Required: true}, {Name: mcp.ParamName("outward_issue"), Description: "Issue key for the outward side", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_delete_issue_link"), Description: "Delete an issue link by link ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("link_id"), Description: "Issue link ID", Required: true}},
	},

	// ── Projects ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("jira_list_projects"), Description: "List all accessible projects",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page (default 50)"}, {Name: mcp.ParamName("query"), Description: "Filter projects by name"}},
	},
	{
		Name: mcp.ToolName("jira_get_project"), Description: "Get details of a specific project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_key"), Description: "Project key or ID", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_list_project_components"), Description: "List components in a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_key"), Description: "Project key or ID", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_list_project_versions"), Description: "List versions (releases) in a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_key"), Description: "Project key or ID", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_list_project_statuses"), Description: "List valid statuses for a project's issue types",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_key"), Description: "Project key or ID", Required: true}},
	},

	// ── Boards & Sprints (Agile API) ────────────────────────────────
	{
		Name: mcp.ToolName("jira_list_boards"), Description: "List all agile boards",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page (default 50)"}, {Name: mcp.ParamName("project_key"), Description: "Filter by project key"}, {Name: mcp.ParamName("type"), Description: "Board type: scrum, kanban, simple"}},
	},
	{
		Name: mcp.ToolName("jira_get_board"), Description: "Get details of an agile board",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("board_id"), Description: "Board ID", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_list_sprints"), Description: "List sprints for a board",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("board_id"), Description: "Board ID", Required: true}, {Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}, {Name: mcp.ParamName("state"), Description: "Filter by state: active, future, closed"}},
	},
	{
		Name: mcp.ToolName("jira_get_sprint"), Description: "Get details of a sprint",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sprint_id"), Description: "Sprint ID", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_create_sprint"), Description: "Create a new sprint",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("board_id"), Description: "Board ID (origin board)", Required: true}, {Name: mcp.ParamName("name"), Description: "Sprint name", Required: true}, {Name: mcp.ParamName("start_date"), Description: "Start date (ISO 8601)"}, {Name: mcp.ParamName("end_date"), Description: "End date (ISO 8601)"}, {Name: mcp.ParamName("goal"), Description: "Sprint goal"}},
	},
	{
		Name: mcp.ToolName("jira_update_sprint"), Description: "Update a sprint's details",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sprint_id"), Description: "Sprint ID", Required: true}, {Name: mcp.ParamName("name"), Description: "Sprint name"}, {Name: mcp.ParamName("state"), Description: "Sprint state: active, future, closed"}, {Name: mcp.ParamName("start_date"), Description: "Start date (ISO 8601)"}, {Name: mcp.ParamName("end_date"), Description: "End date (ISO 8601)"}, {Name: mcp.ParamName("goal"), Description: "Sprint goal"}},
	},
	{
		Name: mcp.ToolName("jira_get_sprint_issues"), Description: "List issues in a sprint",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sprint_id"), Description: "Sprint ID", Required: true}, {Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}, {Name: mcp.ParamName("jql"), Description: "Additional JQL filter"}},
	},
	{
		Name: mcp.ToolName("jira_move_issues_to_sprint"), Description: "Move issues to a sprint",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("sprint_id"), Description: "Sprint ID", Required: true}, {Name: mcp.ParamName("issues"), Description: "Comma-separated issue keys (e.g., PROJ-1,PROJ-2)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_list_board_backlog"), Description: "List issues in the backlog of a board",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("board_id"), Description: "Board ID", Required: true}, {Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}, {Name: mcp.ParamName("jql"), Description: "Additional JQL filter"}},
	},
	{
		Name: mcp.ToolName("jira_get_board_config"), Description: "Get configuration of an agile board (columns, estimation, ranking)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("board_id"), Description: "Board ID", Required: true}},
	},

	// ── Users ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("jira_get_myself"), Description: "Get the current authenticated user's details",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("jira_search_users"), Description: "Search for users by name or email. Use to find account IDs for assignment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (name or email)", Required: true}},
	},
	{
		Name: mcp.ToolName("jira_get_user"), Description: "Get details of a specific user by account ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "User account ID", Required: true}},
	},

	// ── Metadata ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("jira_list_issue_types"), Description: "List all issue types available in the Jira instance",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("jira_list_priorities"), Description: "List all issue priorities",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("jira_list_statuses"), Description: "List all issue statuses",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("jira_list_labels"), Description: "List all labels used across the instance",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("jira_list_fields"), Description: "List all fields (system and custom) available in the instance",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("jira_list_filters"), Description: "Search saved filters",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter_name"), Description: "Filter by name (substring match)"}, {Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}},
	},
	{
		Name: mcp.ToolName("jira_get_filter"), Description: "Get details of a saved filter including its JQL",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter_id"), Description: "Filter ID", Required: true}},
	},

	// ── Worklogs & Info ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("jira_list_worklogs"), Description: "List work logs for an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("start_at"), Description: "Pagination offset"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}},
	},
	{
		Name: mcp.ToolName("jira_add_worklog"), Description: "Add a work log entry to an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_key"), Description: "Issue key (e.g., PROJ-123)", Required: true}, {Name: mcp.ParamName("time_spent"), Description: "Time spent (e.g., 2h, 30m, 1d)", Required: true}, {Name: mcp.ParamName("comment"), Description: "Work log comment (plain text)"}, {Name: mcp.ParamName("started"), Description: "When work started (ISO 8601, defaults to now)"}},
	},
	{
		Name: mcp.ToolName("jira_get_server_info"), Description: "Get Jira instance server info (version, deployment type, URLs)",
		Parameters: []mcp.Parameter{},
	},
}
