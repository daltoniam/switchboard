package datadog

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Logs ──────────────────────────────────────────────────────────
	{
		Name: "datadog_search_logs", Description: "Search Datadog logs for production debugging and observability. Find errors, traces, and events by query string.",
		Parameters: map[string]string{"query": "Log search query (e.g., 'service:nginx status:error')", "from": "Start time (ISO 8601, epoch seconds, or relative like 'now-1h')", "to": "End time (default: now)", "limit": "Max results (default 50, max 1000)", "sort": "Sort order: timestamp (asc) or -timestamp (desc, default)"},
		Required:   []string{"query"},
	},
	{
		Name: "datadog_aggregate_logs", Description: "Aggregate Datadog logs for monitoring analytics (count, sum, avg, etc.). Use for production observability and trend analysis.",
		Parameters: map[string]string{"query": "Log search query", "compute_type": "Aggregation type: count, cardinality, avg, sum, min, max, percentile", "compute_field": "Field to aggregate on (required for non-count types, e.g., @duration)", "group_by": "Field to group by (e.g., service, @http.status_code)", "from": "Start time", "to": "End time"},
		Required:   []string{"query", "compute_type"},
	},

	// ── Metrics ───────────────────────────────────────────────────────
	{
		Name: "datadog_query_metrics", Description: "Query Datadog metrics timeseries data for production monitoring. Analyze performance, CPU, memory, latency, and custom metrics.",
		Parameters: map[string]string{"query": "Metrics query (e.g., 'avg:system.cpu.user{*}')", "from": "Start time (epoch seconds or relative)", "to": "End time"},
		Required:   []string{"query"},
	},
	{
		Name: "datadog_list_active_metrics", Description: "List actively reporting metrics from a given time",
		Parameters: map[string]string{"from": "Start time (epoch seconds, default: now-1h)", "host": "Filter by host", "tag_filter": "Filter by tag (e.g., env:prod)"},
	},
	{
		Name: "datadog_search_metrics", Description: "Search for metrics by name",
		Parameters: map[string]string{"query": "Metric name query (e.g., 'system.cpu')"},
		Required:   []string{"query"},
	},
	{
		Name: "datadog_get_metric_metadata", Description: "Get metadata for a specific metric",
		Parameters: map[string]string{"metric": "Metric name (e.g., system.cpu.user)"},
		Required:   []string{"metric"},
	},

	// ── Monitors ──────────────────────────────────────────────────────
	{
		Name: "datadog_list_monitors", Description: "List Datadog monitors for production alerting and observability. Filter by status, tags, or environment.",
		Parameters: map[string]string{"query": "Filter query (e.g., 'status:alert tag:env:prod')", "page": "Page number (0-based)", "page_size": "Results per page (default 100)"},
	},
	{
		Name: "datadog_search_monitors", Description: "Search Datadog monitors and alerts by query string. Find production monitoring rules and notification policies.",
		Parameters: map[string]string{"query": "Search query (e.g., 'type:metric status:alert')", "page": "Page number", "per_page": "Results per page (default 30)"},
	},
	{
		Name: "datadog_get_monitor", Description: "Get a specific monitor by ID",
		Parameters: map[string]string{"id": "Monitor ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_create_monitor", Description: "Create a new Datadog monitor for production alerting. Set thresholds and notification rules for metrics, logs, or services.",
		Parameters: map[string]string{"name": "Monitor name", "type": "Monitor type: metric alert, service check, event alert, query alert, composite, log alert, etc.", "query": "Monitor query", "message": "Notification message (supports @mentions)", "tags": "Comma-separated tags", "priority": "Priority (1-5)"},
		Required:   []string{"name", "type", "query"},
	},
	{
		Name: "datadog_update_monitor", Description: "Update an existing monitor",
		Parameters: map[string]string{"id": "Monitor ID", "name": "New name", "query": "New query", "message": "New message", "tags": "Comma-separated tags", "priority": "Priority (1-5)"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_delete_monitor", Description: "Delete a monitor",
		Parameters: map[string]string{"id": "Monitor ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_mute_monitor", Description: "Mute a monitor (silence notifications)",
		Parameters: map[string]string{"id": "Monitor ID", "scope": "Scope to mute (e.g., 'host:myhost')", "end": "End timestamp (epoch seconds, omit for indefinite)"},
		Required:   []string{"id"},
	},

	// ── Dashboards ────────────────────────────────────────────────────
	{
		Name: "datadog_list_dashboards", Description: "List all Datadog dashboards for production monitoring and observability visualization",
		Parameters: map[string]string{"filter_shared": "Filter shared dashboards (true/false)", "filter_deleted": "Include deleted (true/false)", "count": "Max results (default 100)", "start": "Offset for pagination"},
	},
	{
		Name: "datadog_get_dashboard", Description: "Get a specific dashboard by ID",
		Parameters: map[string]string{"id": "Dashboard ID (e.g., 'abc-def-ghi')"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_create_dashboard", Description: "Create a new dashboard (JSON body)",
		Parameters: map[string]string{"title": "Dashboard title", "layout_type": "Layout: ordered or free", "description": "Dashboard description", "widgets_json": "JSON array of widget definitions"},
		Required:   []string{"title", "layout_type"},
	},
	{
		Name: "datadog_delete_dashboard", Description: "Delete a dashboard",
		Parameters: map[string]string{"id": "Dashboard ID"},
		Required:   []string{"id"},
	},

	// ── Events ────────────────────────────────────────────────────────
	{
		Name: "datadog_list_events", Description: "List Datadog events for production monitoring. Track deployments, changes, alerts, and system events.",
		Parameters: map[string]string{"query": "Event search query", "from": "Start time", "to": "End time", "limit": "Max results (default 10)", "sort": "Sort: timestamp (asc) or -timestamp (desc)"},
	},
	{
		Name: "datadog_search_events", Description: "Search Datadog events by query. Find production changes, deployments, and system events.",
		Parameters: map[string]string{"query": "Event search query", "from": "Start time", "to": "End time", "limit": "Max results (default 10)", "sort": "Sort: timestamp or -timestamp"},
		Required:   []string{"query"},
	},
	{
		Name: "datadog_get_event", Description: "Get a specific event by ID",
		Parameters: map[string]string{"id": "Event ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_create_event", Description: "Create a new event",
		Parameters: map[string]string{"title": "Event title", "text": "Event text/body", "alert_type": "Type: error, warning, info, success, user_update, recommendation, snapshot", "tags": "Comma-separated tags", "aggregation_key": "Aggregation key for grouping"},
		Required:   []string{"title", "text"},
	},

	// ── Hosts ─────────────────────────────────────────────────────────
	{
		Name: "datadog_list_hosts", Description: "List Datadog hosts (servers and infrastructure). Filter production machines by environment, CPU, load, or tags.",
		Parameters: map[string]string{"filter": "Filter string (e.g., 'env:prod')", "sort_field": "Sort by: apps, cpu, iowait, load, etc.", "sort_dir": "Sort direction: asc or desc", "count": "Max results (default 100)", "from": "Seconds since hosts last reported"},
	},
	{
		Name: "datadog_get_host_totals", Description: "Get total number of hosts",
		Parameters: map[string]string{"from": "Seconds since hosts last reported"},
	},
	{
		Name: "datadog_mute_host", Description: "Mute a host",
		Parameters: map[string]string{"hostname": "Hostname to mute", "message": "Mute reason", "end": "End timestamp (epoch seconds)", "override": "Override existing mute (true/false)"},
		Required:   []string{"hostname"},
	},
	{
		Name: "datadog_unmute_host", Description: "Unmute a host",
		Parameters: map[string]string{"hostname": "Hostname to unmute"},
		Required:   []string{"hostname"},
	},

	// ── Tags ──────────────────────────────────────────────────────────
	{
		Name: "datadog_list_tags", Description: "List all host tags",
		Parameters: map[string]string{"source": "Tag source (e.g., datadog-agent, chef, users)"},
	},
	{
		Name: "datadog_get_host_tags", Description: "Get tags for a specific host",
		Parameters: map[string]string{"hostname": "Hostname", "source": "Tag source"},
		Required:   []string{"hostname"},
	},
	{
		Name: "datadog_create_host_tags", Description: "Add tags to a host",
		Parameters: map[string]string{"hostname": "Hostname", "tags": "Comma-separated tags (e.g., 'env:prod,role:web')", "source": "Tag source"},
		Required:   []string{"hostname", "tags"},
	},
	{
		Name: "datadog_update_host_tags", Description: "Replace all tags on a host",
		Parameters: map[string]string{"hostname": "Hostname", "tags": "Comma-separated tags", "source": "Tag source"},
		Required:   []string{"hostname", "tags"},
	},
	{
		Name: "datadog_delete_host_tags", Description: "Remove tags from a host",
		Parameters: map[string]string{"hostname": "Hostname", "source": "Tag source"},
		Required:   []string{"hostname"},
	},

	// ── SLOs ──────────────────────────────────────────────────────────
	{
		Name: "datadog_list_slos", Description: "List Service Level Objectives",
		Parameters: map[string]string{"ids": "Comma-separated SLO IDs", "query": "Filter query (e.g., 'name:my-slo')", "tags_query": "Filter by tags (e.g., 'env:prod')", "limit": "Max results (default 1000)", "offset": "Pagination offset"},
	},
	{
		Name: "datadog_search_slos", Description: "Search SLOs",
		Parameters: map[string]string{"query": "Search query", "page_size": "Results per page", "page_number": "Page number"},
	},
	{
		Name: "datadog_get_slo", Description: "Get a specific SLO by ID",
		Parameters: map[string]string{"id": "SLO ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_get_slo_history", Description: "Get SLO history data",
		Parameters: map[string]string{"id": "SLO ID", "from": "Start time (epoch seconds)", "to": "End time (epoch seconds)"},
		Required:   []string{"id", "from", "to"},
	},
	{
		Name: "datadog_create_slo", Description: "Create a new SLO",
		Parameters: map[string]string{"name": "SLO name", "type": "Type: metric or monitor", "description": "Description", "target": "Target percentage (e.g., 99.9)", "timeframe": "Timeframe: 7d, 30d, 90d", "monitor_ids": "Comma-separated monitor IDs (for monitor type)", "query_numerator": "Good events query (for metric type)", "query_denominator": "Total events query (for metric type)", "tags": "Comma-separated tags"},
		Required:   []string{"name", "type", "target", "timeframe"},
	},
	{
		Name: "datadog_delete_slo", Description: "Delete a SLO",
		Parameters: map[string]string{"id": "SLO ID"},
		Required:   []string{"id"},
	},

	// ── Downtimes ─────────────────────────────────────────────────────
	{
		Name: "datadog_list_downtimes", Description: "List scheduled downtimes",
		Parameters: map[string]string{"current_only": "Only show current downtimes (true/false)"},
	},
	{
		Name: "datadog_get_downtime", Description: "Get a specific downtime by ID",
		Parameters: map[string]string{"id": "Downtime ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_create_downtime", Description: "Schedule a downtime",
		Parameters: map[string]string{"scope": "Scope (e.g., 'env:prod', 'host:myhost')", "message": "Message/reason", "start": "Start time (epoch seconds, default: now)", "end": "End time (epoch seconds)", "monitor_identifier_type": "Type: id or tags", "monitor_identifier_id": "Monitor ID (when type=id)", "monitor_identifier_tags": "Monitor tags (when type=tags, comma-separated)"},
		Required:   []string{"scope"},
	},
	{
		Name: "datadog_cancel_downtime", Description: "Cancel a scheduled downtime",
		Parameters: map[string]string{"id": "Downtime ID"},
		Required:   []string{"id"},
	},

	// ── Incidents ─────────────────────────────────────────────────────
	{
		Name: "datadog_list_incidents", Description: "List Datadog incidents for production outages and service disruptions. Track severity and incident response.",
		Parameters: map[string]string{"page_size": "Results per page (default 10)", "page_offset": "Pagination offset"},
	},
	{
		Name: "datadog_search_incidents", Description: "Search Datadog incidents by query. Find production outages, disruptions, and postmortems by severity, status, or team.",
		Parameters: map[string]string{"query": "Incident search query (e.g., 'state:active AND severity:SEV-1')", "sort": "Sort order: created or -created (default)", "page_size": "Results per page (default 10)", "page_offset": "Pagination offset"},
		Required:   []string{"query"},
	},
	{
		Name: "datadog_get_incident", Description: "Get details of a specific Datadog incident, including timeline and response data for outage investigation",
		Parameters: map[string]string{"id": "Incident ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_create_incident", Description: "Create a new incident",
		Parameters: map[string]string{"title": "Incident title", "severity": "Severity: SEV-1, SEV-2, SEV-3, SEV-4, SEV-5, UNKNOWN", "customer_impacted": "Customer impacted (true/false)"},
		Required:   []string{"title", "customer_impacted"},
	},
	{
		Name: "datadog_update_incident", Description: "Update an incident",
		Parameters: map[string]string{"id": "Incident ID", "title": "New title", "severity": "New severity", "status": "Status: active, stable, resolved", "customer_impacted": "Customer impacted (true/false)"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_list_incident_attachments", Description: "List attachments for a Datadog incident. View postmortems and linked resources attached to an outage.",
		Parameters: map[string]string{"incident_id": "Incident ID"},
		Required:   []string{"incident_id"},
	},
	{
		Name: "datadog_list_incident_todos", Description: "List todos and action items for a Datadog incident. Track remediation tasks and follow-up work.",
		Parameters: map[string]string{"incident_id": "Incident ID"},
		Required:   []string{"incident_id"},
	},

	// ── Incident Services ────────────────────────────────────────────
	{
		Name: "datadog_list_incident_services", Description: "List Datadog incident services. View services configured for incident management and response.",
		Parameters: map[string]string{"page_size": "Results per page (default 10)", "page_offset": "Pagination offset", "filter": "Filter services by name"},
	},
	{
		Name: "datadog_get_incident_service", Description: "Get details of a specific incident service. Use after list_incident_services.",
		Parameters: map[string]string{"service_id": "Incident service ID"},
		Required:   []string{"service_id"},
	},
	{
		Name: "datadog_create_incident_service", Description: "Create a new incident service for incident management",
		Parameters: map[string]string{"name": "Service name"},
		Required:   []string{"name"},
	},
	{
		Name: "datadog_update_incident_service", Description: "Update an existing incident service",
		Parameters: map[string]string{"service_id": "Incident service ID", "name": "New service name"},
		Required:   []string{"service_id", "name"},
	},
	{
		Name: "datadog_delete_incident_service", Description: "Delete an incident service",
		Parameters: map[string]string{"service_id": "Incident service ID"},
		Required:   []string{"service_id"},
	},

	// ── Incident Teams ───────────────────────────────────────────────
	{
		Name: "datadog_list_incident_teams", Description: "List Datadog incident teams. View teams configured for incident response and on-call rotation.",
		Parameters: map[string]string{"page_size": "Results per page (default 10)", "page_offset": "Pagination offset", "filter": "Filter teams by name"},
	},
	{
		Name: "datadog_get_incident_team", Description: "Get details of a specific incident team. Use after list_incident_teams.",
		Parameters: map[string]string{"team_id": "Incident team ID"},
		Required:   []string{"team_id"},
	},
	{
		Name: "datadog_create_incident_team", Description: "Create a new incident team for incident response",
		Parameters: map[string]string{"name": "Team name"},
		Required:   []string{"name"},
	},
	{
		Name: "datadog_update_incident_team", Description: "Update an existing incident team",
		Parameters: map[string]string{"team_id": "Incident team ID", "name": "New team name"},
		Required:   []string{"team_id", "name"},
	},
	{
		Name: "datadog_delete_incident_team", Description: "Delete an incident team",
		Parameters: map[string]string{"team_id": "Incident team ID"},
		Required:   []string{"team_id"},
	},

	// ── Synthetics ────────────────────────────────────────────────────
	{
		Name: "datadog_list_synthetics_tests", Description: "List synthetic monitoring tests",
		Parameters: map[string]string{"page_size": "Results per page (default 100)", "page_number": "Page number"},
	},
	{
		Name: "datadog_get_synthetics_api_test", Description: "Get a specific synthetics API test",
		Parameters: map[string]string{"id": "Test public ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_get_synthetics_test_result", Description: "Get latest results for a synthetics test",
		Parameters: map[string]string{"id": "Test public ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_trigger_synthetics_tests", Description: "Trigger synthetic tests on demand",
		Parameters: map[string]string{"public_ids": "Comma-separated test public IDs to trigger"},
		Required:   []string{"public_ids"},
	},

	// ── Notebooks ─────────────────────────────────────────────────────
	{
		Name: "datadog_list_notebooks", Description: "List Datadog notebooks",
		Parameters: map[string]string{"query": "Search query", "count": "Max results (default 100)", "start": "Offset for pagination", "sort_field": "Sort by: modified or name", "sort_dir": "Sort direction: asc or desc"},
	},
	{
		Name: "datadog_get_notebook", Description: "Get a specific notebook by ID",
		Parameters: map[string]string{"id": "Notebook ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_create_notebook", Description: "Create a new notebook (JSON body)",
		Parameters: map[string]string{"name": "Notebook name", "cells_json": "JSON array of notebook cells", "time_from": "Time range start", "time_to": "Time range end"},
		Required:   []string{"name"},
	},
	{
		Name: "datadog_delete_notebook", Description: "Delete a notebook",
		Parameters: map[string]string{"id": "Notebook ID"},
		Required:   []string{"id"},
	},

	// ── Users ─────────────────────────────────────────────────────────
	{
		Name: "datadog_list_users", Description: "List users in the organization",
		Parameters: map[string]string{"page_size": "Results per page (default 10)", "page_number": "Page number", "sort": "Sort field (e.g., name, email)", "filter": "Filter string"},
	},
	{
		Name: "datadog_get_user", Description: "Get a specific user by ID",
		Parameters: map[string]string{"id": "User ID"},
		Required:   []string{"id"},
	},

	// ── Teams ─────────────────────────────────────────────────────────
	{
		Name: "datadog_list_teams", Description: "List Datadog teams. View organizational teams, ownership, and membership. Start here for team management.",
		Parameters: map[string]string{"page_size": "Results per page (default 10)", "page_number": "Page number", "filter": "Filter teams by keyword/name", "sort": "Sort: name, -name, user_count, -user_count"},
	},
	{
		Name: "datadog_get_team", Description: "Get details of a specific Datadog team including handle, description, and member count. Use after list_teams.",
		Parameters: map[string]string{"team_id": "Team ID"},
		Required:   []string{"team_id"},
	},
	{
		Name: "datadog_create_team", Description: "Create a new Datadog team",
		Parameters: map[string]string{"name": "Team name", "handle": "Team handle (URL-safe identifier)", "description": "Team description"},
		Required:   []string{"name", "handle"},
	},
	{
		Name: "datadog_update_team", Description: "Update an existing team",
		Parameters: map[string]string{"team_id": "Team ID", "name": "New team name", "handle": "New team handle", "description": "New description"},
		Required:   []string{"team_id", "name", "handle"},
	},
	{
		Name: "datadog_delete_team", Description: "Delete a team",
		Parameters: map[string]string{"team_id": "Team ID"},
		Required:   []string{"team_id"},
	},
	{
		Name: "datadog_list_team_members", Description: "List members of a Datadog team. View who belongs to a team and their roles.",
		Parameters: map[string]string{"team_id": "Team ID", "page_size": "Results per page (default 10)", "page_number": "Page number", "filter": "Filter members by keyword"},
		Required:   []string{"team_id"},
	},
	{
		Name: "datadog_add_team_member", Description: "Add a user to a team",
		Parameters: map[string]string{"team_id": "Team ID", "user_id": "User ID to add", "role": "Role: admin (omit for regular member)"},
		Required:   []string{"team_id", "user_id"},
	},
	{
		Name: "datadog_update_team_member", Description: "Update a team member's role",
		Parameters: map[string]string{"team_id": "Team ID", "user_id": "User ID", "role": "New role: admin (omit for regular member)"},
		Required:   []string{"team_id", "user_id"},
	},
	{
		Name: "datadog_remove_team_member", Description: "Remove a user from a team",
		Parameters: map[string]string{"team_id": "Team ID", "user_id": "User ID to remove"},
		Required:   []string{"team_id", "user_id"},
	},
	{
		Name: "datadog_get_user_team_memberships", Description: "Get all team memberships for a user. Find which teams a user belongs to.",
		Parameters: map[string]string{"user_id": "User ID"},
		Required:   []string{"user_id"},
	},
	{
		Name: "datadog_list_team_links", Description: "List external links for a team. View runbooks, dashboards, and documentation links associated with a team.",
		Parameters: map[string]string{"team_id": "Team ID"},
		Required:   []string{"team_id"},
	},
	{
		Name: "datadog_get_team_link", Description: "Get a specific team link. Use after list_team_links.",
		Parameters: map[string]string{"team_id": "Team ID", "link_id": "Link ID"},
		Required:   []string{"team_id", "link_id"},
	},
	{
		Name: "datadog_create_team_link", Description: "Add a link to a team (runbook, dashboard, documentation)",
		Parameters: map[string]string{"team_id": "Team ID", "label": "Link label", "url": "Link URL"},
		Required:   []string{"team_id", "label", "url"},
	},
	{
		Name: "datadog_update_team_link", Description: "Update a team link",
		Parameters: map[string]string{"team_id": "Team ID", "link_id": "Link ID", "label": "New label", "url": "New URL"},
		Required:   []string{"team_id", "link_id", "label", "url"},
	},
	{
		Name: "datadog_delete_team_link", Description: "Delete a team link",
		Parameters: map[string]string{"team_id": "Team ID", "link_id": "Link ID"},
		Required:   []string{"team_id", "link_id"},
	},
	{
		Name: "datadog_get_team_permission_settings", Description: "Get permission settings for a team. View who can manage membership and edit the team.",
		Parameters: map[string]string{"team_id": "Team ID"},
		Required:   []string{"team_id"},
	},
	{
		Name: "datadog_update_team_permission_setting", Description: "Update a team permission setting",
		Parameters: map[string]string{"team_id": "Team ID", "action": "Permission action: manage_membership or edit", "value": "Permission value: admins, members, organization, user_access_manage, teams_manage"},
		Required:   []string{"team_id", "action", "value"},
	},

	// ── Spans / APM ───────────────────────────────────────────────────
	{
		Name: "datadog_search_spans", Description: "Search Datadog APM spans and distributed traces for production performance debugging. Investigate latency and service dependencies.",
		Parameters: map[string]string{"query": "Span search query (e.g., 'service:web-store resource_name:GET')", "from": "Start time", "to": "End time", "limit": "Max results (default 10)", "sort": "Sort: timestamp or -timestamp"},
		Required:   []string{"query"},
	},

	// ── Software Catalog ──────────────────────────────────────────────
	{
		Name: "datadog_list_services", Description: "List services from the Datadog Software Catalog. View production microservices, dependencies, and ownership.",
		Parameters: map[string]string{"page_size": "Results per page (default 20)", "page_offset": "Pagination offset"},
	},

	// ── On-Call ──────────────────────────────────────────────────────
	{
		Name: "datadog_get_oncall_schedule", Description: "Get a Datadog On-Call schedule with layers, members, and rotation details. View who is on-call and when.",
		Parameters: map[string]string{"schedule_id": "On-Call schedule ID"},
		Required:   []string{"schedule_id"},
	},
	{
		Name: "datadog_create_oncall_schedule", Description: "Create a new On-Call schedule with layers, rotation intervals, and members. Use body_json with Datadog schedule create schema.",
		Parameters: map[string]string{"body_json": "JSON body matching Datadog ScheduleCreateRequest schema: {\"data\":{\"type\":\"schedules\",\"attributes\":{\"name\":\"...\",\"time_zone\":\"...\",\"layers\":[{\"name\":\"...\",\"effective_date\":\"...\",\"rotation_start\":\"...\",\"interval\":{\"days\":7},\"members\":[{\"user\":{\"id\":\"...\"}}]}]}}}"},
		Required:   []string{"body_json"},
	},
	{
		Name: "datadog_update_oncall_schedule", Description: "Update an existing On-Call schedule (layers, members, rotations). Fetch current schedule with get_oncall_schedule first, then modify and submit.",
		Parameters: map[string]string{"schedule_id": "Schedule ID", "body_json": "JSON body matching Datadog ScheduleUpdateRequest schema"},
		Required:   []string{"schedule_id", "body_json"},
	},
	{
		Name: "datadog_delete_oncall_schedule", Description: "Delete an On-Call schedule",
		Parameters: map[string]string{"schedule_id": "Schedule ID"},
		Required:   []string{"schedule_id"},
	},
	{
		Name: "datadog_get_schedule_oncall_user", Description: "Get the current on-call user for a schedule. Find who is on-call right now for a given rotation.",
		Parameters: map[string]string{"schedule_id": "On-Call schedule ID"},
		Required:   []string{"schedule_id"},
	},
	{
		Name: "datadog_get_oncall_escalation_policy", Description: "Get a Datadog On-Call escalation policy with steps, targets, and notification chain. View escalation rules and timing.",
		Parameters: map[string]string{"policy_id": "Escalation policy ID"},
		Required:   []string{"policy_id"},
	},
	{
		Name: "datadog_create_oncall_escalation_policy", Description: "Create a new On-Call escalation policy with steps and targets. Use body_json with Datadog escalation policy create schema.",
		Parameters: map[string]string{"body_json": "JSON body matching Datadog EscalationPolicyCreateRequest schema: {\"data\":{\"type\":\"policies\",\"attributes\":{\"name\":\"...\",\"steps\":[{\"targets\":[{\"type\":\"users\",\"id\":\"...\"}],\"escalate_after_seconds\":300}]}}}"},
		Required:   []string{"body_json"},
	},
	{
		Name: "datadog_update_oncall_escalation_policy", Description: "Update an existing On-Call escalation policy (steps, targets, timing). Fetch current policy with get_oncall_escalation_policy first, then modify and submit.",
		Parameters: map[string]string{"policy_id": "Escalation policy ID", "body_json": "JSON body matching Datadog EscalationPolicyUpdateRequest schema"},
		Required:   []string{"policy_id", "body_json"},
	},
	{
		Name: "datadog_delete_oncall_escalation_policy", Description: "Delete an On-Call escalation policy",
		Parameters: map[string]string{"policy_id": "Escalation policy ID"},
		Required:   []string{"policy_id"},
	},
	{
		Name: "datadog_get_oncall_team_routing_rules", Description: "Get On-Call routing rules for a team. View how pages are routed to schedules and escalation policies.",
		Parameters: map[string]string{"team_id": "Team ID"},
		Required:   []string{"team_id"},
	},
	{
		Name: "datadog_set_oncall_team_routing_rules", Description: "Set (replace) On-Call routing rules for a team. Define how pages are routed to escalation policies. Fetch current rules with get_oncall_team_routing_rules first.",
		Parameters: map[string]string{"team_id": "Team ID", "body_json": "JSON body matching Datadog TeamRoutingRulesRequest schema: {\"data\":{\"type\":\"team_routing_rules\",\"attributes\":{\"rules\":[{\"policy_id\":\"...\",\"urgency\":\"high\"}]}}}"},
		Required:   []string{"team_id", "body_json"},
	},
	{
		Name: "datadog_get_team_oncall_users", Description: "Get the current on-call users for a team. Find who is on-call right now across all team schedules.",
		Parameters: map[string]string{"team_id": "Team ID"},
		Required:   []string{"team_id"},
	},

	// ── On-Call Paging ───────────────────────────────────────────────
	{
		Name: "datadog_create_oncall_page", Description: "Create a new On-Call page to alert responders. Page a team or user for production incidents and urgent issues.",
		Parameters: map[string]string{"title": "Page title", "urgency": "Urgency: low or high (default high)", "target_id": "Target identifier (team handle, team ID, or user ID)", "target_type": "Target type: team_handle, team_id, or user_id (default team_handle)", "description": "Page description with details", "tags": "Comma-separated tags"},
		Required:   []string{"title", "target_id"},
	},
	{
		Name: "datadog_acknowledge_oncall_page", Description: "Acknowledge an On-Call page to indicate responder awareness",
		Parameters: map[string]string{"page_id": "Page UUID"},
		Required:   []string{"page_id"},
	},
	{
		Name: "datadog_escalate_oncall_page", Description: "Escalate an On-Call page to the next responder in the escalation policy",
		Parameters: map[string]string{"page_id": "Page UUID"},
		Required:   []string{"page_id"},
	},
	{
		Name: "datadog_resolve_oncall_page", Description: "Resolve an On-Call page to mark the issue as handled",
		Parameters: map[string]string{"page_id": "Page UUID"},
		Required:   []string{"page_id"},
	},

	// ── IP Ranges ─────────────────────────────────────────────────────
	{
		Name: "datadog_get_ip_ranges", Description: "Get Datadog IP address ranges used for inbound/outbound traffic",
		Parameters: map[string]string{},
	},
}
