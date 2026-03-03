package linear

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Issues ────────────────────────────────────────────────────────
	{
		Name: "linear_list_issues", Description: "List Linear issues with optional filters",
		Parameters: map[string]string{"team": "Filter by team name or key", "assignee": "Filter by assignee name or 'me'", "state": "Filter by state name (e.g., 'In Progress', 'Done')", "label": "Filter by label name", "priority": "Filter by priority (1=urgent, 2=high, 3=normal, 4=low)", "project": "Filter by project name", "cycle": "Filter by cycle name or number", "first": "Max results (default 50)", "after": "Pagination cursor"},
	},
	{
		Name: "linear_search_issues", Description: "Full-text search Linear issues",
		Parameters: map[string]string{"query": "Search query text", "first": "Max results (default 50)", "after": "Pagination cursor"},
		Required:   []string{"query"},
	},
	{
		Name: "linear_get_issue", Description: "Get a specific issue by identifier (e.g., ENG-123)",
		Parameters: map[string]string{"id": "Issue identifier (e.g., ENG-123) or UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_create_issue", Description: "Create a new issue",
		Parameters: map[string]string{"title": "Issue title", "team": "Team name or key", "description": "Description (markdown)", "assignee": "Assignee name or email", "priority": "Priority (0=none, 1=urgent, 2=high, 3=normal, 4=low)", "state": "Workflow state name", "labels": "Comma-separated label names", "project": "Project name", "cycle": "Cycle name or number", "estimate": "Story point estimate", "due_date": "Due date (YYYY-MM-DD)", "parent_id": "Parent issue ID for sub-issues"},
		Required:   []string{"title", "team"},
	},
	{
		Name: "linear_update_issue", Description: "Update an existing issue",
		Parameters: map[string]string{"id": "Issue identifier (e.g., ENG-123) or UUID", "title": "New title", "description": "New description", "assignee": "Assignee name or email", "priority": "Priority (0-4)", "state": "Workflow state name", "labels": "Comma-separated label names", "project": "Project name", "estimate": "Story point estimate", "due_date": "Due date (YYYY-MM-DD)"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_archive_issue", Description: "Archive an issue",
		Parameters: map[string]string{"id": "Issue identifier or UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_unarchive_issue", Description: "Unarchive an issue",
		Parameters: map[string]string{"id": "Issue identifier or UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_list_issue_comments", Description: "List comments on an issue",
		Parameters: map[string]string{"id": "Issue identifier or UUID", "first": "Max results (default 50)"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_create_comment", Description: "Create a comment on an issue",
		Parameters: map[string]string{"issue_id": "Issue identifier or UUID", "body": "Comment body (markdown)"},
		Required:   []string{"issue_id", "body"},
	},
	{
		Name: "linear_update_comment", Description: "Update a comment",
		Parameters: map[string]string{"id": "Comment UUID", "body": "New comment body (markdown)"},
		Required:   []string{"id", "body"},
	},
	{
		Name: "linear_delete_comment", Description: "Delete a comment",
		Parameters: map[string]string{"id": "Comment UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_list_issue_relations", Description: "List relations for an issue (blocks, blocked by, related, duplicate)",
		Parameters: map[string]string{"id": "Issue identifier or UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_create_issue_relation", Description: "Create a relation between two issues",
		Parameters: map[string]string{"issue_id": "Source issue identifier or UUID", "related_issue_id": "Related issue identifier or UUID", "type": "Relation type: blocks, blocked_by, related, duplicate"},
		Required:   []string{"issue_id", "related_issue_id", "type"},
	},
	{
		Name: "linear_delete_issue_relation", Description: "Delete an issue relation",
		Parameters: map[string]string{"id": "Relation UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_list_issue_labels", Description: "List labels on a specific issue",
		Parameters: map[string]string{"id": "Issue identifier or UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_list_attachments", Description: "List attachments on an issue",
		Parameters: map[string]string{"issue_id": "Issue identifier or UUID", "first": "Max results (default 25)"},
		Required:   []string{"issue_id"},
	},
	{
		Name: "linear_create_attachment", Description: "Create an attachment (link) on an issue",
		Parameters: map[string]string{"issue_id": "Issue identifier or UUID", "title": "Attachment title", "url": "Attachment URL", "subtitle": "Subtitle"},
		Required:   []string{"issue_id", "url"},
	},

	// ── Projects ──────────────────────────────────────────────────────
	{
		Name: "linear_list_projects", Description: "List projects with optional filters",
		Parameters: map[string]string{"team": "Filter by team name or key", "state": "Filter by state: planned, started, paused, completed, canceled", "first": "Max results (default 50)", "after": "Pagination cursor"},
	},
	{
		Name: "linear_search_projects", Description: "Full-text search Linear projects",
		Parameters: map[string]string{"query": "Search query", "first": "Max results (default 50)"},
		Required:   []string{"query"},
	},
	{
		Name: "linear_get_project", Description: "Get a specific project by ID or slug",
		Parameters: map[string]string{"id": "Project UUID or slug"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_create_project", Description: "Create a project",
		Parameters: map[string]string{"name": "Project name", "team": "Team name or key", "description": "Description (markdown)", "state": "State: planned, started, paused, completed, canceled", "lead": "Project lead name or email", "target_date": "Target date (YYYY-MM-DD)", "start_date": "Start date (YYYY-MM-DD)"},
		Required:   []string{"name", "team"},
	},
	{
		Name: "linear_update_project", Description: "Update a project",
		Parameters: map[string]string{"id": "Project UUID", "name": "New name", "description": "New description", "state": "State: planned, started, paused, completed, canceled", "lead": "Project lead name or email", "target_date": "Target date", "start_date": "Start date"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_archive_project", Description: "Archive a project",
		Parameters: map[string]string{"id": "Project UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_list_project_updates", Description: "List status updates for a project",
		Parameters: map[string]string{"project_id": "Project UUID", "first": "Max results (default 10)"},
		Required:   []string{"project_id"},
	},
	{
		Name: "linear_create_project_update", Description: "Create a project status update",
		Parameters: map[string]string{"project_id": "Project UUID", "body": "Update body (markdown)", "health": "Health: onTrack, atRisk, offTrack"},
		Required:   []string{"project_id", "body"},
	},
	{
		Name: "linear_list_project_milestones", Description: "List milestones for a project",
		Parameters: map[string]string{"project_id": "Project UUID", "first": "Max results (default 50)"},
		Required:   []string{"project_id"},
	},
	{
		Name: "linear_create_project_milestone", Description: "Create a project milestone",
		Parameters: map[string]string{"project_id": "Project UUID", "name": "Milestone name", "description": "Description", "target_date": "Target date (YYYY-MM-DD)", "sort_order": "Sort order (number)"},
		Required:   []string{"project_id", "name"},
	},

	// ── Cycles ────────────────────────────────────────────────────────
	{
		Name: "linear_list_cycles", Description: "List cycles with optional filters",
		Parameters: map[string]string{"team": "Filter by team name or key", "first": "Max results (default 50)", "after": "Pagination cursor"},
	},
	{
		Name: "linear_get_cycle", Description: "Get a specific cycle by ID",
		Parameters: map[string]string{"id": "Cycle UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_create_cycle", Description: "Create a cycle",
		Parameters: map[string]string{"team": "Team name or key", "name": "Cycle name", "description": "Description", "starts_at": "Start date (ISO 8601)", "ends_at": "End date (ISO 8601)"},
		Required:   []string{"team", "starts_at", "ends_at"},
	},
	{
		Name: "linear_update_cycle", Description: "Update a cycle",
		Parameters: map[string]string{"id": "Cycle UUID", "name": "New name", "description": "New description", "starts_at": "Start date", "ends_at": "End date"},
		Required:   []string{"id"},
	},

	// ── Teams ─────────────────────────────────────────────────────────
	{
		Name: "linear_list_teams", Description: "List all teams in the workspace",
		Parameters: map[string]string{"first": "Max results (default 50)"},
	},
	{
		Name: "linear_get_team", Description: "Get a specific team by ID, name, or key",
		Parameters: map[string]string{"id": "Team UUID, name, or key"},
		Required:   []string{"id"},
	},

	// ── Users ─────────────────────────────────────────────────────────
	{
		Name: "linear_viewer", Description: "Get the currently authenticated user",
		Parameters: map[string]string{},
	},
	{
		Name: "linear_list_users", Description: "List users in the workspace",
		Parameters: map[string]string{"first": "Max results (default 50)"},
	},
	{
		Name: "linear_get_user", Description: "Get a specific user by ID, name, or email",
		Parameters: map[string]string{"id": "User UUID, display name, or email"},
		Required:   []string{"id"},
	},

	// ── Labels ────────────────────────────────────────────────────────
	{
		Name: "linear_list_labels", Description: "List issue labels in the workspace",
		Parameters: map[string]string{"team": "Filter by team name or key", "first": "Max results (default 100)"},
	},
	{
		Name: "linear_create_label", Description: "Create an issue label",
		Parameters: map[string]string{"name": "Label name", "color": "Hex color (e.g., #ff0000)", "team": "Team name or key (omit for workspace label)", "description": "Description"},
		Required:   []string{"name"},
	},
	{
		Name: "linear_update_label", Description: "Update an issue label",
		Parameters: map[string]string{"id": "Label UUID", "name": "New name", "color": "New hex color", "description": "New description"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_delete_label", Description: "Delete an issue label",
		Parameters: map[string]string{"id": "Label UUID"},
		Required:   []string{"id"},
	},

	// ── Workflow States ───────────────────────────────────────────────
	{
		Name: "linear_list_workflow_states", Description: "List workflow states for a team",
		Parameters: map[string]string{"team": "Team name or key", "first": "Max results (default 50)"},
	},
	{
		Name: "linear_create_workflow_state", Description: "Create a workflow state",
		Parameters: map[string]string{"team": "Team name or key", "name": "State name", "type": "Type: triage, backlog, unstarted, started, completed, canceled", "color": "Hex color", "description": "Description", "position": "Sort position (number)"},
		Required:   []string{"team", "name", "type", "color"},
	},

	// ── Documents ─────────────────────────────────────────────────────
	{
		Name: "linear_list_documents", Description: "List documents in the workspace",
		Parameters: map[string]string{"project": "Filter by project name", "first": "Max results (default 50)"},
	},
	{
		Name: "linear_search_documents", Description: "Full-text search documents",
		Parameters: map[string]string{"query": "Search query", "first": "Max results (default 25)"},
		Required:   []string{"query"},
	},
	{
		Name: "linear_get_document", Description: "Get a specific document by ID",
		Parameters: map[string]string{"id": "Document UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_create_document", Description: "Create a document",
		Parameters: map[string]string{"title": "Document title", "content": "Content (markdown)", "project": "Associated project name or UUID", "icon": "Icon emoji"},
		Required:   []string{"title"},
	},
	{
		Name: "linear_update_document", Description: "Update a document",
		Parameters: map[string]string{"id": "Document UUID", "title": "New title", "content": "New content (markdown)", "icon": "New icon emoji"},
		Required:   []string{"id"},
	},

	// ── Initiatives ───────────────────────────────────────────────────
	{
		Name: "linear_list_initiatives", Description: "List initiatives",
		Parameters: map[string]string{"first": "Max results (default 50)"},
	},
	{
		Name: "linear_get_initiative", Description: "Get a specific initiative by ID",
		Parameters: map[string]string{"id": "Initiative UUID"},
		Required:   []string{"id"},
	},
	{
		Name: "linear_create_initiative", Description: "Create an initiative",
		Parameters: map[string]string{"name": "Initiative name", "description": "Description (markdown)", "target_date": "Target date (YYYY-MM-DD)", "status": "Status: Planned, Active, Completed"},
		Required:   []string{"name"},
	},
	{
		Name: "linear_update_initiative", Description: "Update an initiative",
		Parameters: map[string]string{"id": "Initiative UUID", "name": "New name", "description": "New description", "target_date": "Target date", "status": "Status: Planned, Active, Completed"},
		Required:   []string{"id"},
	},

	// ── Favorites ─────────────────────────────────────────────────────
	{
		Name: "linear_list_favorites", Description: "List favorites for the authenticated user",
		Parameters: map[string]string{},
	},
	{
		Name: "linear_create_favorite", Description: "Add an item to favorites",
		Parameters: map[string]string{"issue_id": "Issue UUID", "project_id": "Project UUID", "cycle_id": "Cycle UUID", "custom_view_id": "Custom view UUID"},
	},
	{
		Name: "linear_delete_favorite", Description: "Remove a favorite",
		Parameters: map[string]string{"id": "Favorite UUID"},
		Required:   []string{"id"},
	},

	// ── Webhooks ──────────────────────────────────────────────────────
	{
		Name: "linear_list_webhooks", Description: "List webhooks in the workspace",
		Parameters: map[string]string{},
	},
	{
		Name: "linear_create_webhook", Description: "Create a webhook",
		Parameters: map[string]string{"url": "Webhook URL", "team": "Team name or key (omit for all teams)", "label": "Label to filter events", "resource_types": "Comma-separated resource types: Issue, Comment, Project, Cycle, IssueLabel, etc.", "all_public_teams": "Subscribe to all public teams (true/false)"},
		Required:   []string{"url"},
	},
	{
		Name: "linear_delete_webhook", Description: "Delete a webhook",
		Parameters: map[string]string{"id": "Webhook UUID"},
		Required:   []string{"id"},
	},

	// ── Notifications ─────────────────────────────────────────────────
	{
		Name: "linear_list_notifications", Description: "List notifications for the authenticated user",
		Parameters: map[string]string{"first": "Max results (default 50)"},
	},

	// ── Templates ─────────────────────────────────────────────────────
	{
		Name: "linear_list_templates", Description: "List issue templates",
		Parameters: map[string]string{},
	},

	// ── Organization ──────────────────────────────────────────────────
	{
		Name: "linear_get_organization", Description: "Get the current organization details",
		Parameters: map[string]string{},
	},

	// ── Custom Views ──────────────────────────────────────────────────
	{
		Name: "linear_list_custom_views", Description: "List custom views (saved filters)",
		Parameters: map[string]string{"first": "Max results (default 50)"},
	},
	{
		Name: "linear_create_custom_view", Description: "Create a custom view (saved filter)",
		Parameters: map[string]string{"name": "View name", "description": "Description", "team": "Team name or key", "filter_state": "Filter by state names (comma-separated)", "filter_assignee": "Filter by assignee (name or 'me')", "filter_label": "Filter by label name", "filter_priority": "Filter by priority (1-4)"},
		Required:   []string{"name"},
	},

	// ── Rate Limit ────────────────────────────────────────────────────
	{
		Name: "linear_rate_limit", Description: "Get current API rate limit status",
		Parameters: map[string]string{},
	},
}
