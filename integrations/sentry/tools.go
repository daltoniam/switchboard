package sentry

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Organizations ────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_get_organization"), Description: "Get details of the Sentry organization",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("sentry_list_org_projects"), Description: "List all projects in the organization",
		Parameters: map[string]string{"cursor": "Pagination cursor"},
	},
	{
		Name: mcp.ToolName("sentry_list_org_teams"), Description: "List all teams in the organization",
		Parameters: map[string]string{"cursor": "Pagination cursor"},
	},
	{
		Name: mcp.ToolName("sentry_list_org_members"), Description: "List members of the organization",
		Parameters: map[string]string{"cursor": "Pagination cursor"},
	},
	{
		Name: mcp.ToolName("sentry_get_org_member"), Description: "Get details of an organization member",
		Parameters: map[string]string{"member_id": "Member ID (or 'me' for current user)"},
		Required:   []string{"member_id"},
	},
	{
		Name: mcp.ToolName("sentry_list_org_repos"), Description: "List repositories connected to the organization",
		Parameters: map[string]string{"cursor": "Pagination cursor"},
	},
	{
		Name: mcp.ToolName("sentry_resolve_short_id"), Description: "Resolve a Sentry short ID (e.g., PROJECT-123) to full error issue details. Use to look up a bug by its short reference.",
		Parameters: map[string]string{"short_id": "Short ID (e.g., PROJECT-123)"},
		Required:   []string{"short_id"},
	},

	// ── Projects ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_projects"), Description: "List all projects accessible to the auth token",
		Parameters: map[string]string{"cursor": "Pagination cursor"},
	},
	{
		Name: mcp.ToolName("sentry_get_project"), Description: "Get details of a specific project",
		Parameters: map[string]string{"project": "Project slug"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_update_project"), Description: "Update a project's settings",
		Parameters: map[string]string{"project": "Project slug", "name": "New name", "slug": "New slug", "platform": "Platform (e.g., python, javascript)", "isBookmarked": "Bookmark project (true/false)"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_delete_project"), Description: "Delete a project",
		Parameters: map[string]string{"project": "Project slug"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_create_project"), Description: "Create a new project under a team",
		Parameters: map[string]string{"team": "Team slug", "name": "Project name", "slug": "Project slug (optional, auto-generated from name)", "platform": "Platform (e.g., python, javascript)"},
		Required:   []string{"team", "name"},
	},
	{
		Name: mcp.ToolName("sentry_list_project_keys"), Description: "List a project's client keys (DSN)",
		Parameters: map[string]string{"project": "Project slug"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_list_project_envs"), Description: "List a project's environments",
		Parameters: map[string]string{"project": "Project slug"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_list_project_tags"), Description: "List tags for a project",
		Parameters: map[string]string{"project": "Project slug"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_get_project_stats"), Description: "Get error and crash event count statistics for a project. Use to monitor error rate and volume.",
		Parameters: map[string]string{"project": "Project slug", "stat": "Stat type: received, rejected, blacklisted, generated (default: received)", "since": "Unix timestamp for start", "until": "Unix timestamp for end", "resolution": "Resolution in seconds (e.g., 3600 for hourly)"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_list_project_hooks"), Description: "List service hooks for a project",
		Parameters: map[string]string{"project": "Project slug"},
		Required:   []string{"project"},
	},

	// ── Teams ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_get_team"), Description: "Get details of a specific team",
		Parameters: map[string]string{"team": "Team slug"},
		Required:   []string{"team"},
	},
	{
		Name: mcp.ToolName("sentry_create_team"), Description: "Create a new team in the organization",
		Parameters: map[string]string{"name": "Team name", "slug": "Team slug (optional)"},
		Required:   []string{"name"},
	},
	{
		Name: mcp.ToolName("sentry_delete_team"), Description: "Delete a team",
		Parameters: map[string]string{"team": "Team slug"},
		Required:   []string{"team"},
	},
	{
		Name: mcp.ToolName("sentry_list_team_projects"), Description: "List projects belonging to a team",
		Parameters: map[string]string{"team": "Team slug", "cursor": "Pagination cursor"},
		Required:   []string{"team"},
	},

	// ── Issues & Events ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_issues"), Description: "List errors and exceptions for a project. Start here for error tracking, debugging, and finding unresolved bugs or crashes.",
		Parameters: map[string]string{"project": "Project slug", "query": "Search query (e.g., 'is:unresolved', 'assigned:me')", "cursor": "Pagination cursor", "sort": "Sort: date, new, freq, user (default: date)", "statsPeriod": "Stats period: '' (default), '24h', or '14d'"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_get_issue"), Description: "Get details of a specific error or exception issue, including stacktrace and debugging context",
		Parameters: map[string]string{"issue_id": "Issue ID"},
		Required:   []string{"issue_id"},
	},
	{
		Name: mcp.ToolName("sentry_update_issue"), Description: "Update an error issue (resolve, assign, triage, etc.)",
		Parameters: map[string]string{"issue_id": "Issue ID", "status": "Status: resolved, unresolved, ignored", "assignedTo": "Assign to user (email or username, empty to unassign)", "hasSeen": "Mark as seen (true/false)", "isBookmarked": "Bookmark (true/false)", "isSubscribed": "Subscribe (true/false)", "isPublic": "Make public (true/false)"},
		Required:   []string{"issue_id"},
	},
	{
		Name: mcp.ToolName("sentry_delete_issue"), Description: "Delete an issue",
		Parameters: map[string]string{"issue_id": "Issue ID"},
		Required:   []string{"issue_id"},
	},
	{
		Name: mcp.ToolName("sentry_list_issue_events"), Description: "List error occurrences and crash events for a specific issue",
		Parameters: map[string]string{"issue_id": "Issue ID", "cursor": "Pagination cursor"},
		Required:   []string{"issue_id"},
	},
	{
		Name: mcp.ToolName("sentry_list_issue_hashes"), Description: "List hashes (fingerprints) for an issue",
		Parameters: map[string]string{"issue_id": "Issue ID", "cursor": "Pagination cursor"},
		Required:   []string{"issue_id"},
	},
	{
		Name: mcp.ToolName("sentry_get_issue_tag_values"), Description: "Get tag value distribution for an issue",
		Parameters: map[string]string{"issue_id": "Issue ID", "tag_name": "Tag key (e.g., browser, os, url, environment)"},
		Required:   []string{"issue_id", "tag_name"},
	},
	{
		Name: mcp.ToolName("sentry_list_project_events"), Description: "List error and exception events for a project. Use to investigate crashes and debug production issues.",
		Parameters: map[string]string{"project": "Project slug", "cursor": "Pagination cursor"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_get_event"), Description: "Get details of a specific event",
		Parameters: map[string]string{"project": "Project slug", "event_id": "Event ID"},
		Required:   []string{"project", "event_id"},
	},
	{
		Name: mcp.ToolName("sentry_list_org_issues"), Description: "List error and exception issues across the entire organization. Search all projects for bugs, crashes, and unresolved problems.",
		Parameters: map[string]string{"query": "Search query (e.g., 'is:unresolved level:error')", "project": "Filter by project slug", "cursor": "Pagination cursor", "sort": "Sort: date, new, freq, user", "statsPeriod": "Stats period: '' (default), '24h', or '14d'"},
	},

	// ── Releases ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_releases"), Description: "List releases for the organization",
		Parameters: map[string]string{"query": "Filter releases by version", "project": "Filter by project slug", "cursor": "Pagination cursor"},
	},
	{
		Name: mcp.ToolName("sentry_get_release"), Description: "Get details of a specific release",
		Parameters: map[string]string{"version": "Release version identifier"},
		Required:   []string{"version"},
	},
	{
		Name: mcp.ToolName("sentry_create_release"), Description: "Create a new release",
		Parameters: map[string]string{"version": "Release version", "ref": "Git ref (commit SHA or tag)", "url": "URL for the release", "projects": "Comma-separated project slugs", "dateReleased": "Release date (ISO 8601)"},
		Required:   []string{"version", "projects"},
	},
	{
		Name: mcp.ToolName("sentry_delete_release"), Description: "Delete a release",
		Parameters: map[string]string{"version": "Release version"},
		Required:   []string{"version"},
	},
	{
		Name: mcp.ToolName("sentry_list_release_commits"), Description: "List commits in a release",
		Parameters: map[string]string{"version": "Release version", "cursor": "Pagination cursor"},
		Required:   []string{"version"},
	},
	{
		Name: mcp.ToolName("sentry_list_release_deploys"), Description: "List deploys for a release",
		Parameters: map[string]string{"version": "Release version"},
		Required:   []string{"version"},
	},
	{
		Name: mcp.ToolName("sentry_create_deploy"), Description: "Create a deploy for a release",
		Parameters: map[string]string{"version": "Release version", "environment": "Environment name (e.g., production)", "name": "Deploy name", "url": "Deploy URL", "dateStarted": "Start time (ISO 8601)", "dateFinished": "Finish time (ISO 8601)"},
		Required:   []string{"version", "environment"},
	},
	{
		Name: mcp.ToolName("sentry_list_release_files"), Description: "List files (artifacts) in a release",
		Parameters: map[string]string{"version": "Release version", "cursor": "Pagination cursor"},
		Required:   []string{"version"},
	},

	// ── Alerts ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_metric_alerts"), Description: "List metric alert rules for monitoring thresholds and warnings across the organization",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("sentry_get_metric_alert"), Description: "Get a specific metric alert rule",
		Parameters: map[string]string{"alert_rule_id": "Alert rule ID"},
		Required:   []string{"alert_rule_id"},
	},
	{
		Name: mcp.ToolName("sentry_delete_metric_alert"), Description: "Delete a metric alert rule",
		Parameters: map[string]string{"alert_rule_id": "Alert rule ID"},
		Required:   []string{"alert_rule_id"},
	},
	{
		Name: mcp.ToolName("sentry_list_issue_alerts"), Description: "List error alert rules that trigger notifications for a project",
		Parameters: map[string]string{"project": "Project slug"},
		Required:   []string{"project"},
	},
	{
		Name: mcp.ToolName("sentry_get_issue_alert"), Description: "Get a specific issue alert rule",
		Parameters: map[string]string{"project": "Project slug", "alert_rule_id": "Alert rule ID"},
		Required:   []string{"project", "alert_rule_id"},
	},
	{
		Name: mcp.ToolName("sentry_delete_issue_alert"), Description: "Delete an issue alert rule",
		Parameters: map[string]string{"project": "Project slug", "alert_rule_id": "Alert rule ID"},
		Required:   []string{"project", "alert_rule_id"},
	},

	// ── Monitors (Cron) ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_monitors"), Description: "List cron monitors for the organization",
		Parameters: map[string]string{"project": "Filter by project slug", "cursor": "Pagination cursor"},
	},
	{
		Name: mcp.ToolName("sentry_get_monitor"), Description: "Get a specific cron monitor",
		Parameters: map[string]string{"project": "Project slug", "monitor_id": "Monitor ID or slug"},
		Required:   []string{"project", "monitor_id"},
	},
	{
		Name: mcp.ToolName("sentry_delete_monitor"), Description: "Delete a cron monitor",
		Parameters: map[string]string{"project": "Project slug", "monitor_id": "Monitor ID or slug"},
		Required:   []string{"project", "monitor_id"},
	},

	// ── Discover ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_saved_queries"), Description: "List saved Discover queries",
		Parameters: map[string]string{"cursor": "Pagination cursor", "sortBy": "Sort by: dateCreated, dateUpdated, name, myqueries (default: dateUpdated)"},
	},
	{
		Name: mcp.ToolName("sentry_get_saved_query"), Description: "Get a specific saved Discover query",
		Parameters: map[string]string{"query_id": "Saved query ID"},
		Required:   []string{"query_id"},
	},
	{
		Name: mcp.ToolName("sentry_delete_saved_query"), Description: "Delete a saved Discover query",
		Parameters: map[string]string{"query_id": "Saved query ID"},
		Required:   []string{"query_id"},
	},

	// ── Replays ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_replays"), Description: "List session replay recordings. Use to visually reproduce and investigate specific user sessions.",
		Parameters: map[string]string{"query": "Search query", "cursor": "Pagination cursor", "limit": "Max results (default 50)", "statsPeriod": "Stats period: '' (default), '24h', or '14d'"},
	},
	{
		Name: mcp.ToolName("sentry_get_replay"), Description: "Get details of a specific replay",
		Parameters: map[string]string{"replay_id": "Replay ID"},
		Required:   []string{"replay_id"},
	},
	{
		Name: mcp.ToolName("sentry_delete_replay"), Description: "Delete a replay",
		Parameters: map[string]string{"replay_id": "Replay ID"},
		Required:   []string{"replay_id"},
	},
}
