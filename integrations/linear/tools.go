package linear

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Issues ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_list_issues"), Description: "List Linear issues (tickets/bugs/tasks) with optional filters. Start here for filtered queries (by assignee, state, label, project). Use list_workflow_states to discover valid state names.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Filter by team name or key"}, {Name: mcp.ParamName("assignee"), Description: "Filter by assignee name or 'me'"}, {Name: mcp.ParamName("state"), Description: "Filter by state name (e.g., 'In Progress', 'Done')"}, {Name: mcp.ParamName("label"), Description: "Filter by label name"}, {Name: mcp.ParamName("priority"), Description: "Filter by priority (1=urgent, 2=high, 3=normal, 4=low)"}, {Name: mcp.ParamName("project"), Description: "Filter by project name"}, {Name: mcp.ParamName("cycle"), Description: "Filter by cycle name or number"}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}, {Name: mcp.ParamName("after"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("linear_search_issues"), Description: "Full-text search Linear issues (tickets/bugs) by keyword. Start here to find issues by text. For filtering by assignee, state, or label, prefer list_issues instead.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query text", Required: true}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}, {Name: mcp.ParamName("after"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("linear_get_issue"), Description: "Get a specific issue with full detail. Accepts issue ID (UUID) or identifier (e.g., ENG-123).",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Issue identifier (e.g., ENG-123) or UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_create_issue"), Description: "Create a new issue (ticket/bug/task). Requires team_id — use list_teams to find it. Use list_workflow_states to discover valid state names.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("title"), Description: "Issue title", Required: true}, {Name: mcp.ParamName("team"), Description: "Team name or key", Required: true}, {Name: mcp.ParamName("description"), Description: "Description (markdown)"}, {Name: mcp.ParamName("assignee"), Description: "Assignee name or email"}, {Name: mcp.ParamName("priority"), Description: "Priority (0=none, 1=urgent, 2=high, 3=normal, 4=low)"}, {Name: mcp.ParamName("state"), Description: "Workflow state name"}, {Name: mcp.ParamName("labels"), Description: "Comma-separated label names"}, {Name: mcp.ParamName("project"), Description: "Project name"}, {Name: mcp.ParamName("milestone"), Description: "Project milestone name or UUID"}, {Name: mcp.ParamName("cycle"), Description: "Cycle name or number"}, {Name: mcp.ParamName("estimate"), Description: "Story point estimate"}, {Name: mcp.ParamName("due_date"), Description: "Due date (YYYY-MM-DD)"}, {Name: mcp.ParamName("parent_id"), Description: "Parent issue ID for sub-issues"}},
	},
	{
		Name: mcp.ToolName("linear_update_issue"), Description: "Update an existing issue. Accepts issue ID (UUID) or identifier (e.g., ENG-123). Use list_workflow_states to discover valid state names. Use list_teams to find team names for transfers.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Issue identifier (e.g., ENG-123) or UUID", Required: true}, {Name: mcp.ParamName("title"), Description: "New title"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("assignee"), Description: "Assignee name or email"}, {Name: mcp.ParamName("priority"), Description: "Priority (0-4)"}, {Name: mcp.ParamName("state"), Description: "Workflow state name"}, {Name: mcp.ParamName("team"), Description: "Team name or key (moves issue to this team)"}, {Name: mcp.ParamName("labels"), Description: "Comma-separated label names"}, {Name: mcp.ParamName("project"), Description: "Project name"}, {Name: mcp.ParamName("milestone"), Description: "Project milestone name or UUID"}, {Name: mcp.ParamName("estimate"), Description: "Story point estimate"}, {Name: mcp.ParamName("due_date"), Description: "Due date (YYYY-MM-DD)"}},
	},
	{
		Name: mcp.ToolName("linear_archive_issue"), Description: "Archive an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Issue identifier or UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_unarchive_issue"), Description: "Unarchive an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Issue identifier or UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_list_issue_comments"), Description: "List comments on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Issue identifier or UUID", Required: true}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_create_comment"), Description: "Create a comment on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue identifier or UUID", Required: true}, {Name: mcp.ParamName("body"), Description: "Comment body (markdown)", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_update_comment"), Description: "Update a comment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Comment UUID", Required: true}, {Name: mcp.ParamName("body"), Description: "New comment body (markdown)", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_delete_comment"), Description: "Delete a comment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Comment UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_list_issue_relations"), Description: "List relations for an issue (blocks, blocked by, related, duplicate)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Issue identifier or UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_create_issue_relation"), Description: "Create a relation between two issues",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Source issue identifier or UUID", Required: true}, {Name: mcp.ParamName("related_issue_id"), Description: "Related issue identifier or UUID", Required: true}, {Name: mcp.ParamName("type"), Description: "Relation type: blocks, blocked_by, related, duplicate", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_delete_issue_relation"), Description: "Delete an issue relation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Relation UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_list_issue_labels"), Description: "List labels on a specific issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Issue identifier or UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_list_attachments"), Description: "List attachments on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue identifier or UUID", Required: true}, {Name: mcp.ParamName("first"), Description: "Max results (default 25)"}},
	},
	{
		Name: mcp.ToolName("linear_create_attachment"), Description: "Create an attachment (link) on an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue identifier or UUID", Required: true}, {Name: mcp.ParamName("title"), Description: "Attachment title"}, {Name: mcp.ParamName("url"),

		// ── Projects ──────────────────────────────────────────────────────
		Description: "Attachment URL", Required: true}, {Name: mcp.ParamName("subtitle"), Description: "Subtitle"}},
	},

	{
		Name: mcp.ToolName("linear_list_projects"), Description: "List projects with optional filters. Start here for project status queries.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Filter by team name or key"}, {Name: mcp.ParamName("state"), Description: "Filter by state: planned, started, paused, completed, canceled"}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}, {Name: mcp.ParamName("after"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("linear_search_projects"), Description: "Find Linear projects by name or keyword. Returns project IDs needed by get_project and list_project_updates. Start here when you know the project name.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Project name or keyword", Required: true}, {Name: mcp.ParamName("first"), Description: "Max results (default 10)"}},
	},
	{
		Name: mcp.ToolName("linear_get_project"), Description: "Get a specific project with full detail including progress, members, recent status updates, and milestones. Accepts project UUID or slug. Use search_projects to find the ID first.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Project UUID or slug", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_create_project"), Description: "Create a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Project name", Required: true}, {Name: mcp.ParamName("team"), Description: "Team name or key", Required: true}, {Name: mcp.ParamName("description"), Description: "Description (markdown)"}, {Name: mcp.ParamName("state"), Description: "State: planned, started, paused, completed, canceled"}, {Name: mcp.ParamName("lead"), Description: "Project lead name or email"}, {Name: mcp.ParamName("target_date"), Description: "Target date (YYYY-MM-DD)"}, {Name: mcp.ParamName("start_date"), Description: "Start date (YYYY-MM-DD)"}},
	},
	{
		Name: mcp.ToolName("linear_update_project"), Description: "Update a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Project UUID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("state"), Description: "State: planned, started, paused, completed, canceled"}, {Name: mcp.ParamName("lead"), Description: "Project lead name or email"}, {Name: mcp.ParamName("target_date"), Description: "Target date"}, {Name: mcp.ParamName("start_date"), Description: "Start date"}},
	},
	{
		Name: mcp.ToolName("linear_archive_project"), Description: "Archive a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Project UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_list_project_updates"), Description: "List status updates (health reports) for a project. Requires project UUID — use search_projects to find it. For a quick summary, get_project already includes the 5 most recent updates.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project UUID", Required: true}, {Name: mcp.ParamName("first"), Description: "Max results (default 10)"}},
	},
	{
		Name: mcp.ToolName("linear_create_project_update"), Description: "Create a project status update",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project UUID", Required: true}, {Name: mcp.ParamName("body"), Description: "Update body (markdown)", Required: true}, {Name: mcp.ParamName("health"), Description: "Health: onTrack, atRisk, offTrack"}},
	},
	{
		Name: mcp.ToolName("linear_list_project_milestones"), Description: "List milestones for a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project UUID", Required: true}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_create_project_milestone"), Description: "Create a project milestone",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project UUID", Required: true}, {Name: mcp.ParamName("name"), Description: "Milestone name", Required: true}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName(

		// ── Cycles ────────────────────────────────────────────────────────
		"target_date"), Description: "Target date (YYYY-MM-DD)"}, {Name: mcp.ParamName("sort_order"), Description: "Sort order (number)"}},
	},

	{
		Name: mcp.ToolName("linear_list_cycles"), Description: "List cycles (sprints) with optional filters. Use to find the current sprint, then get_cycle for its issues.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Filter by team name or key"}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}, {Name: mcp.ParamName("after"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("linear_get_cycle"), Description: "Get a specific cycle with its issues. Use after list_cycles to drill into a sprint.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Cycle UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_create_cycle"), Description: "Create a cycle",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Team name or key", Required: true}, {Name: mcp.ParamName("name"), Description: "Cycle name"}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("starts_at"), Description: "Start date (ISO 8601)", Required: true}, {Name: mcp.ParamName("ends_at"), Description: "End date (ISO 8601)", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_update_cycle"), Description: "Update a cycle",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Cycle UUID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("description"), Description:

		// ── Teams ─────────────────────────────────────────────────────────
		"New description"}, {Name: mcp.ParamName("starts_at"), Description: "Start date"}, {Name: mcp.ParamName("ends_at"), Description: "End date"}},
	},

	{
		Name: mcp.ToolName("linear_list_teams"), Description: "List all teams in the workspace. Use to discover team IDs needed by create_issue and list_workflow_states.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_get_team"), Description: "Get a specific team with members and settings. Accepts team UUID, name, or key.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Team UUID, name, or key", Required: true}},
	},

	// ── Users ─────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_viewer"), Description: "Get the currently authenticated user. Use to find your user ID for filtering assigned issues via list_issues.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("linear_list_users"), Description: "List users in the workspace. Use to find user IDs for assignee filtering.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_get_user"), Description: "Get a specific user with assigned issues count. Accepts user UUID, display name, or email.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "User UUID, display name, or email", Required: true}},
	},

	// ── Labels ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_list_labels"), Description: "List issue labels in the workspace. Use to discover valid label names for list_issues filtering or create_issue.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Filter by team name or key"}, {Name: mcp.ParamName("first"), Description: "Max results (default 100)"}},
	},
	{
		Name: mcp.ToolName("linear_create_label"), Description: "Create an issue label",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Label name", Required: true}, {Name: mcp.ParamName("color"), Description: "Hex color (e.g., #ff0000)"}, {Name: mcp.ParamName("team"), Description: "Team name or key (omit for workspace label)"}, {Name: mcp.ParamName("description"), Description: "Description"}},
	},
	{
		Name: mcp.ToolName("linear_update_label"), Description: "Update an issue label",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Label UUID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("color"), Description: "New hex color"}, {Name: mcp.ParamName("description"), Description: "New description"}},
	},
	{
		Name: mcp.ToolName("linear_delete_label"), Description: "Delete an issue label",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Label UUID", Required: true}},
	},

	// ── Workflow States ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_list_workflow_states"), Description: "List workflow states for a team. Use before list_issues, create_issue, or update_issue to discover valid state names.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Team name or key"}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_create_workflow_state"), Description: "Create a workflow state",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Team name or key", Required: true}, {Name: mcp.ParamName("name"), Description: "State name", Required: true}, {Name: mcp.ParamName("type"), Description: "Type: triage, backlog, unstarted, started, completed, canceled", Required: true}, {Name: mcp.ParamName(

		// ── Documents ─────────────────────────────────────────────────────
		"color"), Description: "Hex color", Required: true}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("position"), Description: "Sort position (number)"}},
	},

	{
		Name: mcp.ToolName("linear_list_documents"), Description: "List documents in the workspace. Filter by project to find project-specific docs.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Filter by project name"}, {Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_search_documents"), Description: "Full-text search documents by keyword. For browsing by project, use list_documents with project filter.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query", Required: true}, {Name: mcp.ParamName("first"), Description: "Max results (default 25)"}},
	},
	{
		Name: mcp.ToolName("linear_get_document"), Description: "Get a specific document by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Document UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_create_document"), Description: "Create a document",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("title"), Description: "Document title", Required: true}, {Name: mcp.ParamName("content"), Description: "Content (markdown)"}, {Name: mcp.ParamName("project"), Description: "Associated project name or UUID"}, {Name: mcp.ParamName("icon"), Description: "Icon emoji"}},
	},
	{
		Name: mcp.ToolName("linear_update_document"), Description: "Update a document",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Document UUID", Required: true}, {Name: mcp.ParamName("title"), Description: "New title"}, {Name: mcp.ParamName("content"),

		// ── Initiatives ───────────────────────────────────────────────────
		Description: "New content (markdown)"}, {Name: mcp.ParamName("icon"), Description: "New icon emoji"}},
	},

	{
		Name: mcp.ToolName("linear_list_initiatives"), Description: "List initiatives",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_get_initiative"), Description: "Get a specific initiative by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Initiative UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("linear_create_initiative"), Description: "Create an initiative",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Initiative name", Required: true}, {Name: mcp.ParamName("description"), Description: "Description (markdown)"}, {Name: mcp.ParamName("target_date"), Description: "Target date (YYYY-MM-DD)"}, {Name: mcp.ParamName("status"), Description: "Status: Planned, Active, Completed"}},
	},
	{
		Name: mcp.ToolName("linear_update_initiative"), Description: "Update an initiative",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Initiative UUID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.

		// ── Favorites ─────────────────────────────────────────────────────
		ParamName("target_date"), Description: "Target date"}, {Name: mcp.ParamName("status"), Description: "Status: Planned, Active, Completed"}},
	},

	{
		Name: mcp.ToolName("linear_list_favorites"), Description: "List favorites for the authenticated user",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("linear_create_favorite"), Description: "Add an item to favorites",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue UUID"}, {Name: mcp.ParamName("project_id"), Description: "Project UUID"}, {Name: mcp.ParamName("cycle_id"), Description: "Cycle UUID"}, {Name: mcp.ParamName("custom_view_id"), Description: "Custom view UUID"}},
	},
	{
		Name: mcp.ToolName("linear_delete_favorite"), Description: "Remove a favorite",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Favorite UUID", Required: true}},
	},

	// ── Webhooks ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_list_webhooks"), Description: "List webhooks in the workspace",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("linear_create_webhook"), Description: "Create a webhook",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("url"), Description: "Webhook URL", Required: true}, {Name: mcp.ParamName("team"), Description: "Team name or key (omit for all teams)"}, {Name: mcp.ParamName("label"), Description: "Label to filter events"}, {Name: mcp.ParamName("resource_types"), Description: "Comma-separated resource types: Issue, Comment, Project, Cycle, IssueLabel, etc."}, {Name: mcp.ParamName("all_public_teams"), Description: "Subscribe to all public teams (true/false)"}},
	},
	{
		Name: mcp.ToolName("linear_delete_webhook"), Description: "Delete a webhook",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Webhook UUID", Required: true}},
	},

	// ── Notifications ─────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_list_notifications"), Description: "List notifications for the authenticated user",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},

	// ── Templates ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_list_templates"), Description: "List issue templates",
		Parameters: []mcp.Parameter{},
	},

	// ── Organization ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_get_organization"), Description: "Get the current organization details",
		Parameters: []mcp.Parameter{},
	},

	// ── Custom Views ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("linear_list_custom_views"), Description: "List custom views (saved filters)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("first"), Description: "Max results (default 50)"}},
	},
	{
		Name: mcp.ToolName("linear_create_custom_view"), Description: "Create a custom view (saved filter)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "View name", Required: true}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("team"), Description: "Team name or key"}, {Name: mcp.ParamName("filter_state"), Description: "Filter by state names (comma-separated)"}, {Name: mcp.ParamName("filter_assignee"), Description:

		// ── Rate Limit ────────────────────────────────────────────────────
		"Filter by assignee (name or 'me')"}, {Name: mcp.ParamName("filter_label"), Description: "Filter by label name"}, {Name: mcp.ParamName("filter_priority"), Description: "Filter by priority (1-4)"}},
	},

	{
		Name: mcp.ToolName("linear_rate_limit"), Description: "Get current API rate limit status",
		Parameters: []mcp.Parameter{},
	},
}
