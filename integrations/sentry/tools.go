package sentry

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Organizations ────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_get_organization"), Description: "Get details of the Sentry organization",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("sentry_list_org_projects"), Description: "List all projects in the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_list_org_teams"), Description: "List all teams in the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_list_org_members"), Description: "List members of the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_get_org_member"), Description: "Get details of an organization member",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("member_id"), Description: "Member ID (or 'me' for current user)", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_org_repos"), Description: "List repositories connected to the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_resolve_short_id"), Description: "Resolve a Sentry short ID (e.g., PROJECT-123) to full error issue details. Use to look up a bug by its short reference.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("short_id"), Description: "Short ID (e.g., PROJECT-123)", Required: true}},
	},

	// ── Projects ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_projects"), Description: "List all projects accessible to the auth token",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_get_project"), Description: "Get details of a specific project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_update_project"), Description: "Update a project's settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("slug"), Description: "New slug"}, {Name: mcp.ParamName("platform"), Description: "Platform (e.g., python, javascript)"}, {Name: mcp.ParamName("isBookmarked"), Description: "Bookmark project (true/false)"}},
	},
	{
		Name: mcp.ToolName("sentry_delete_project"), Description: "Delete a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_create_project"), Description: "Create a new project under a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("name"), Description: "Project name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Project slug (optional, auto-generated from name)"}, {Name: mcp.ParamName("platform"), Description: "Platform (e.g., python, javascript)"}},
	},
	{
		Name: mcp.ToolName("sentry_list_project_keys"), Description: "List a project's client keys (DSN)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_project_envs"), Description: "List a project's environments",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_project_tags"), Description: "List tags for a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_get_project_stats"), Description: "Get error and crash event count statistics for a project. Use to monitor error rate and volume.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("stat"), Description: "Stat type: received, rejected, blacklisted, generated (default: received)"}, {Name: mcp.ParamName("since"), Description: "Unix timestamp for start"}, {Name: mcp.ParamName("until"), Description: "Unix timestamp for end"}, {Name: mcp.ParamName("resolution"), Description: "Resolution in seconds (e.g., 3600 for hourly)"}},
	},
	{
		Name: mcp.ToolName("sentry_list_project_hooks"), Description: "List service hooks for a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}},
	},

	// ── Teams ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_get_team"), Description: "Get details of a specific team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Team slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_create_team"), Description: "Create a new team in the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Team name", Required: true}, {Name: mcp.ParamName("slug"), Description: "Team slug (optional)"}},
	},
	{
		Name: mcp.ToolName("sentry_delete_team"), Description: "Delete a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Team slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_team_projects"), Description: "List projects belonging to a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team"), Description: "Team slug", Required: true}, {Name: mcp.ParamName("cursor"),

		// ── Issues & Events ──────────────────────────────────────────────
		Description: "Pagination cursor"}},
	},

	{
		Name: mcp.ToolName("sentry_list_issues"), Description: "List errors and exceptions for a project. Start here for error tracking, debugging, and finding unresolved bugs or crashes.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("query"), Description: "Search query (e.g., 'is:unresolved', 'assigned:me')"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}, {Name: mcp.ParamName("sort"), Description: "Sort: date, new, freq, user (default: date)"}, {Name: mcp.ParamName("statsPeriod"), Description: "Stats period: '' (default), '24h', or '14d'"}},
	},
	{
		Name: mcp.ToolName("sentry_get_issue"), Description: "Get details of a specific error or exception issue, including stacktrace and debugging context",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_update_issue"), Description: "Update an error issue (resolve, assign, triage, etc.)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue ID", Required: true}, {Name: mcp.ParamName("status"), Description: "Status: resolved, unresolved, ignored"}, {Name: mcp.ParamName("assignedTo"), Description: "Assign to user (email or username, empty to unassign)"}, {Name: mcp.ParamName("hasSeen"), Description: "Mark as seen (true/false)"}, {Name: mcp.ParamName("isBookmarked"), Description: "Bookmark (true/false)"}, {Name: mcp.ParamName("isSubscribed"), Description: "Subscribe (true/false)"}, {Name: mcp.ParamName("isPublic"), Description: "Make public (true/false)"}},
	},
	{
		Name: mcp.ToolName("sentry_delete_issue"), Description: "Delete an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_issue_events"), Description: "List error occurrences and crash events for a specific issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue ID", Required: true}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_list_issue_hashes"), Description: "List hashes (fingerprints) for an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue ID", Required: true}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_get_issue_tag_values"), Description: "Get tag value distribution for an issue",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("issue_id"), Description: "Issue ID", Required: true}, {Name: mcp.ParamName("tag_name"), Description: "Tag key (e.g., browser, os, url, environment)", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_project_events"), Description: "List error and exception events for a project. Use to investigate crashes and debug production issues.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_get_event"), Description: "Get details of a specific event",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("event_id"), Description: "Event ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_org_issues"), Description: "List error and exception issues across the entire organization. Search all projects for bugs, crashes, and unresolved problems.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (e.g., 'is:unresolved level:error')"}, {Name: mcp.ParamName("project"), Description: "Filter by project slug"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}, {Name: mcp.ParamName(

		// ── Releases ─────────────────────────────────────────────────────
		"sort"), Description: "Sort: date, new, freq, user"}, {Name: mcp.ParamName("statsPeriod"), Description: "Stats period: '' (default), '24h', or '14d'"}},
	},

	{
		Name: mcp.ToolName("sentry_list_releases"), Description: "List releases for the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Filter releases by version"}, {Name: mcp.ParamName("project"), Description: "Filter by project slug"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_get_release"), Description: "Get details of a specific release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("version"), Description: "Release version identifier", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_create_release"), Description: "Create a new release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("version"), Description: "Release version", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (commit SHA or tag)"}, {Name: mcp.ParamName("url"), Description: "URL for the release"}, {Name: mcp.ParamName("projects"), Description: "Comma-separated project slugs", Required: true}, {Name: mcp.ParamName("dateReleased"), Description: "Release date (ISO 8601)"}},
	},
	{
		Name: mcp.ToolName("sentry_delete_release"), Description: "Delete a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("version"), Description: "Release version", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_release_commits"), Description: "List commits in a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("version"), Description: "Release version", Required: true}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_list_release_deploys"), Description: "List deploys for a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("version"), Description: "Release version", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_create_deploy"), Description: "Create a deploy for a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("version"), Description: "Release version", Required: true}, {Name: mcp.ParamName("environment"), Description: "Environment name (e.g., production)", Required: true}, {Name: mcp.ParamName("name"), Description: "Deploy name"}, {Name: mcp.ParamName("url"), Description: "Deploy URL"}, {Name: mcp.ParamName("dateStarted"), Description: "Start time (ISO 8601)"}, {Name: mcp.ParamName("dateFinished"), Description: "Finish time (ISO 8601)"}},
	},
	{
		Name: mcp.ToolName("sentry_list_release_files"), Description: "List files (artifacts) in a release",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("version"), Description: "Release version", Required: true}, {Name: mcp.ParamName("cursor"), Description:

		// ── Alerts ───────────────────────────────────────────────────────
		"Pagination cursor"}},
	},

	{
		Name: mcp.ToolName("sentry_list_metric_alerts"), Description: "List metric alert rules for monitoring thresholds and warnings across the organization",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("sentry_get_metric_alert"), Description: "Get a specific metric alert rule",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("alert_rule_id"), Description: "Alert rule ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_delete_metric_alert"), Description: "Delete a metric alert rule",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("alert_rule_id"), Description: "Alert rule ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_list_issue_alerts"), Description: "List error alert rules that trigger notifications for a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_get_issue_alert"), Description: "Get a specific issue alert rule",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("alert_rule_id"), Description: "Alert rule ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_delete_issue_alert"), Description: "Delete an issue alert rule",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("alert_rule_id"), Description: "Alert rule ID",

		// ── Monitors (Cron) ──────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("sentry_list_monitors"), Description: "List cron monitors for the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Filter by project slug"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}},
	},
	{
		Name: mcp.ToolName("sentry_get_monitor"), Description: "Get a specific cron monitor",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("monitor_id"), Description: "Monitor ID or slug", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_delete_monitor"), Description: "Delete a cron monitor",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project"), Description: "Project slug", Required: true}, {Name: mcp.ParamName("monitor_id"), Description: "Monitor ID or slug",

		// ── Discover ─────────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("sentry_list_saved_queries"), Description: "List saved Discover queries",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}, {Name: mcp.ParamName("sortBy"), Description: "Sort by: dateCreated, dateUpdated, name, myqueries (default: dateUpdated)"}},
	},
	{
		Name: mcp.ToolName("sentry_get_saved_query"), Description: "Get a specific saved Discover query",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query_id"), Description: "Saved query ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_delete_saved_query"), Description: "Delete a saved Discover query",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query_id"), Description: "Saved query ID", Required: true}},
	},

	// ── Replays ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("sentry_list_replays"), Description: "List session replay recordings. Use to visually reproduce and investigate specific user sessions.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}, {Name: mcp.ParamName("limit"), Description: "Max results (default 50)"}, {Name: mcp.ParamName("statsPeriod"), Description: "Stats period: '' (default), '24h', or '14d'"}},
	},
	{
		Name: mcp.ToolName("sentry_get_replay"), Description: "Get details of a specific replay",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("replay_id"), Description: "Replay ID", Required: true}},
	},
	{
		Name: mcp.ToolName("sentry_delete_replay"), Description: "Delete a replay",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("replay_id"), Description: "Replay ID", Required: true}},
	},
}
