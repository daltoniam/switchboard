package datadog

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Logs ──────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_search_logs"), Description: "Search Datadog logs for production debugging and observability. Find errors, traces, and events by query string.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Log search query (e.g., 'service:nginx status:error')", Required: true}, {Name: mcp.ParamName("from"), Description: "Start time (ISO 8601, epoch seconds, or relative like 'now-1h')"}, {Name: mcp.ParamName("to"), Description: "End time (default: now)"}, {Name: mcp.ParamName("limit"), Description: "Max results (default 50, max 1000)"}, {Name: mcp.ParamName("sort"), Description: "Sort order: timestamp (asc) or -timestamp (desc, default)"}},
	},
	{
		Name: mcp.ToolName("datadog_aggregate_logs"), Description: "Aggregate Datadog logs for monitoring analytics (count, sum, avg, etc.). Use for production observability and trend analysis.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Log search query", Required: true}, {Name: mcp.ParamName("compute_type"), Description: "Aggregation type: count, cardinality, avg, sum, min, max, percentile", Required: true}, {Name: mcp.ParamName("compute_field"), Description: "Field to aggregate on (required for non-count types, e.g., @duration)"}, {Name: mcp.ParamName("group_by"), Description:

		// ── Metrics ───────────────────────────────────────────────────────
		"Field to group by (e.g., service, @http.status_code)"}, {Name: mcp.ParamName("from"), Description: "Start time"}, {Name: mcp.ParamName("to"), Description: "End time"}},
	},

	{
		Name: mcp.ToolName("datadog_query_metrics"), Description: "Query Datadog metrics timeseries data for production monitoring. Analyze performance, CPU, memory, latency, and custom metrics.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Metrics query (e.g., 'avg:system.cpu.user{*}')", Required: true}, {Name: mcp.ParamName("from"), Description: "Start time (epoch seconds or relative)"}, {Name: mcp.ParamName("to"), Description: "End time"}},
	},
	{
		Name: mcp.ToolName("datadog_list_active_metrics"), Description: "List actively reporting metrics from a given time",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("from"), Description: "Start time (epoch seconds, default: now-1h)"}, {Name: mcp.ParamName("host"), Description: "Filter by host"}, {Name: mcp.ParamName("tag_filter"), Description: "Filter by tag (e.g., env:prod)"}},
	},
	{
		Name: mcp.ToolName("datadog_search_metrics"), Description: "Search for metrics by name",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Metric name query (e.g., 'system.cpu')", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_metric_metadata"), Description: "Get metadata for a specific metric",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("metric"), Description: "Metric name (e.g., system.cpu.user)", Required: true}},
	},

	// ── Monitors ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_monitors"), Description: "List Datadog monitors for production alerting and observability. Filter by status, tags, or environment.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Filter query (e.g., 'status:alert tag:env:prod')"}, {Name: mcp.ParamName("page"), Description: "Page number (0-based)"}, {Name: mcp.ParamName("page_size"), Description: "Results per page (default 100)"}},
	},
	{
		Name: mcp.ToolName("datadog_search_monitors"), Description: "Search Datadog monitors and alerts by query string. Find production monitoring rules and notification policies.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (e.g., 'type:metric status:alert')"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page (default 30)"}},
	},
	{
		Name: mcp.ToolName("datadog_get_monitor"), Description: "Get a specific monitor by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Monitor ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_monitor"), Description: "Create a new Datadog monitor for production alerting. Set thresholds and notification rules for metrics, logs, or services.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Monitor name", Required: true}, {Name: mcp.ParamName("type"), Description: "Monitor type: metric alert, service check, event alert, query alert, composite, log alert, etc.", Required: true}, {Name: mcp.ParamName("query"), Description: "Monitor query", Required: true}, {Name: mcp.ParamName("message"), Description: "Notification message (supports @mentions)"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}, {Name: mcp.ParamName("priority"), Description: "Priority (1-5)"}},
	},
	{
		Name: mcp.ToolName("datadog_update_monitor"), Description: "Update an existing monitor",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Monitor ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("query"), Description: "New query"}, {Name: mcp.ParamName("message"), Description: "New message"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}, {Name: mcp.ParamName("priority"), Description: "Priority (1-5)"}},
	},
	{
		Name: mcp.ToolName("datadog_delete_monitor"), Description: "Delete a monitor",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Monitor ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_mute_monitor"), Description: "Mute a monitor (silence notifications)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Monitor ID", Required: true}, {Name: mcp.ParamName("scope"), Description: "Scope to mute (e.g., 'host:myhost')"}, {Name: mcp.ParamName("end"),

		// ── Dashboards ────────────────────────────────────────────────────
		Description: "End timestamp (epoch seconds, omit for indefinite)"}},
	},

	{
		Name: mcp.ToolName("datadog_list_dashboards"), Description: "List all Datadog dashboards for production monitoring and observability visualization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter_shared"), Description: "Filter shared dashboards (true/false)"}, {Name: mcp.ParamName("filter_deleted"), Description: "Include deleted (true/false)"}, {Name: mcp.ParamName("count"), Description: "Max results (default 100)"}, {Name: mcp.ParamName("start"), Description: "Offset for pagination"}},
	},
	{
		Name: mcp.ToolName("datadog_get_dashboard"), Description: "Get a specific dashboard by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Dashboard ID (e.g., 'abc-def-ghi')", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_dashboard"), Description: "Create a new dashboard (JSON body)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("title"), Description: "Dashboard title", Required: true}, {Name: mcp.ParamName("layout_type"), Description: "Layout: ordered or free", Required: true}, {Name: mcp.ParamName("description"), Description: "Dashboard description"}, {Name: mcp.ParamName("widgets_json"), Description: "JSON array of widget definitions"}},
	},
	{
		Name: mcp.ToolName("datadog_delete_dashboard"), Description: "Delete a dashboard",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Dashboard ID", Required: true}},
	},

	// ── Events ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_events"), Description: "List Datadog events for production monitoring. Track deployments, changes, alerts, and system events.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Event search query"}, {Name: mcp.ParamName("from"), Description: "Start time"}, {Name: mcp.ParamName("to"), Description: "End time"}, {Name: mcp.ParamName("limit"), Description: "Max results (default 10)"}, {Name: mcp.ParamName("sort"), Description: "Sort: timestamp (asc) or -timestamp (desc)"}},
	},
	{
		Name: mcp.ToolName("datadog_search_events"), Description: "Search Datadog events by query. Find production changes, deployments, and system events.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Event search query", Required: true}, {Name: mcp.ParamName("from"), Description: "Start time"}, {Name: mcp.ParamName("to"), Description: "End time"}, {Name: mcp.ParamName("limit"), Description: "Max results (default 10)"}, {Name: mcp.ParamName("sort"), Description: "Sort: timestamp or -timestamp"}},
	},
	{
		Name: mcp.ToolName("datadog_get_event"), Description: "Get a specific event by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Event ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_event"), Description: "Create a new event",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("title"), Description: "Event title", Required: true}, {Name: mcp.ParamName("text"), Description: "Event text/body", Required: true}, {Name: mcp.ParamName("alert_type"), Description: "Type: error, warning, info, success, user_update, recommendation, snapshot"}, {Name: mcp.ParamName(

		// ── Hosts ─────────────────────────────────────────────────────────
		"tags"), Description: "Comma-separated tags"}, {Name: mcp.ParamName("aggregation_key"), Description: "Aggregation key for grouping"}},
	},

	{
		Name: mcp.ToolName("datadog_list_hosts"), Description: "List Datadog hosts (servers and infrastructure). Filter production machines by environment, CPU, load, or tags.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: "Filter string (e.g., 'env:prod')"}, {Name: mcp.ParamName("sort_field"), Description: "Sort by: apps, cpu, iowait, load, etc."}, {Name: mcp.ParamName("sort_dir"), Description: "Sort direction: asc or desc"}, {Name: mcp.ParamName("count"), Description: "Max results (default 100)"}, {Name: mcp.ParamName("from"), Description: "Seconds since hosts last reported"}},
	},
	{
		Name: mcp.ToolName("datadog_get_host_totals"), Description: "Get total number of hosts",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("from"), Description: "Seconds since hosts last reported"}},
	},
	{
		Name: mcp.ToolName("datadog_mute_host"), Description: "Mute a host",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("hostname"), Description: "Hostname to mute", Required: true}, {Name: mcp.ParamName("message"), Description: "Mute reason"}, {Name: mcp.ParamName("end"), Description: "End timestamp (epoch seconds)"}, {Name: mcp.ParamName("override"), Description: "Override existing mute (true/false)"}},
	},
	{
		Name: mcp.ToolName("datadog_unmute_host"), Description: "Unmute a host",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("hostname"), Description: "Hostname to unmute", Required: true}},
	},

	// ── Tags ──────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_tags"), Description: "List all host tags",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("source"), Description: "Tag source (e.g., datadog-agent, chef, users)"}},
	},
	{
		Name: mcp.ToolName("datadog_get_host_tags"), Description: "Get tags for a specific host",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("hostname"), Description: "Hostname", Required: true}, {Name: mcp.ParamName("source"), Description: "Tag source"}},
	},
	{
		Name: mcp.ToolName("datadog_create_host_tags"), Description: "Add tags to a host",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("hostname"), Description: "Hostname", Required: true}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags (e.g., 'env:prod,role:web')", Required: true}, {Name: mcp.ParamName("source"), Description: "Tag source"}},
	},
	{
		Name: mcp.ToolName("datadog_update_host_tags"), Description: "Replace all tags on a host",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("hostname"), Description: "Hostname", Required: true}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags", Required: true}, {Name: mcp.ParamName("source"), Description: "Tag source"}},
	},
	{
		Name: mcp.ToolName("datadog_delete_host_tags"), Description: "Remove tags from a host",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("hostname"), Description: "Hostname", Required: true}, {Name: mcp.ParamName("source"),

		// ── SLOs ──────────────────────────────────────────────────────────
		Description: "Tag source"}},
	},

	{
		Name: mcp.ToolName("datadog_list_slos"), Description: "List Service Level Objectives",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("ids"), Description: "Comma-separated SLO IDs"}, {Name: mcp.ParamName("query"), Description: "Filter query (e.g., 'name:my-slo')"}, {Name: mcp.ParamName("tags_query"), Description: "Filter by tags (e.g., 'env:prod')"}, {Name: mcp.ParamName("limit"), Description: "Max results (default 1000)"}, {Name: mcp.ParamName("offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("datadog_search_slos"), Description: "Search SLOs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query"}, {Name: mcp.ParamName("page_size"), Description: "Results per page"}, {Name: mcp.ParamName("page_number"), Description: "Page number"}},
	},
	{
		Name: mcp.ToolName("datadog_get_slo"), Description: "Get a specific SLO by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "SLO ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_slo_history"), Description: "Get SLO history data",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "SLO ID", Required: true}, {Name: mcp.ParamName("from"), Description: "Start time (epoch seconds)", Required: true}, {Name: mcp.ParamName("to"), Description: "End time (epoch seconds)", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_slo"), Description: "Create a new SLO",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "SLO name", Required: true}, {Name: mcp.ParamName("type"), Description: "Type: metric or monitor", Required: true}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("target"), Description: "Target percentage (e.g., 99.9)", Required: true}, {Name: mcp.ParamName("timeframe"), Description: "Timeframe: 7d, 30d, 90d", Required: true}, {Name: mcp.ParamName("monitor_ids"), Description: "Comma-separated monitor IDs (for monitor type)"}, {Name: mcp.ParamName("query_numerator"), Description: "Good events query (for metric type)"}, {Name: mcp.ParamName("query_denominator"), Description:

		// ── Downtimes ─────────────────────────────────────────────────────
		"Total events query (for metric type)"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}},
	},
	{
		Name: mcp.ToolName("datadog_delete_slo"), Description: "Delete a SLO",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "SLO ID", Required: true}},
	},

	{
		Name: mcp.ToolName("datadog_list_downtimes"), Description: "List scheduled downtimes",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("current_only"), Description: "Only show current downtimes (true/false)"}},
	},
	{
		Name: mcp.ToolName("datadog_get_downtime"), Description: "Get a specific downtime by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Downtime ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_downtime"), Description: "Schedule a downtime",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("scope"), Description: "Scope (e.g., 'env:prod', 'host:myhost')", Required: true}, {Name: mcp.ParamName("message"), Description: "Message/reason"}, {Name: mcp.ParamName("start"), Description: "Start time (epoch seconds, default: now)"}, {Name: mcp.ParamName("end"), Description: "End time (epoch seconds)"}, {Name: mcp.ParamName("monitor_identifier_type"), Description: "Type: id or tags"}, {Name: mcp.ParamName("monitor_identifier_id"), Description: "Monitor ID (when type=id)"}, {Name: mcp.ParamName("monitor_identifier_tags"), Description: "Monitor tags (when type=tags, comma-separated)"}},
	},
	{
		Name: mcp.ToolName("datadog_cancel_downtime"), Description: "Cancel a scheduled downtime",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Downtime ID", Required: true}},
	},

	// ── Incidents ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_incidents"), Description: "List Datadog incidents for production outages and service disruptions. Track severity and incident response.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_size"), Description: "Results per page (default 10)"}, {Name: mcp.ParamName("page_offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("datadog_search_incidents"), Description: "Search Datadog incidents by query. Find production outages, disruptions, and postmortems by severity, status, or team.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Incident search query (e.g., 'state:active AND severity:SEV-1')", Required: true}, {Name: mcp.ParamName("sort"), Description: "Sort order: created or -created (default)"}, {Name: mcp.ParamName("page_size"), Description: "Results per page (default 10)"}, {Name: mcp.ParamName("page_offset"), Description: "Pagination offset"}},
	},
	{
		Name: mcp.ToolName("datadog_get_incident"), Description: "Get details of a specific Datadog incident, including timeline and response data for outage investigation",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Incident ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_incident"), Description: "Create a new incident",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("title"), Description: "Incident title", Required: true}, {Name: mcp.ParamName("severity"), Description: "Severity: SEV-1, SEV-2, SEV-3, SEV-4, SEV-5, UNKNOWN"}, {Name: mcp.ParamName("customer_impacted"), Description: "Customer impacted (true/false)", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_update_incident"), Description: "Update an incident",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Incident ID", Required: true}, {Name: mcp.ParamName("title"), Description: "New title"}, {Name: mcp.ParamName("severity"), Description: "New severity"}, {Name: mcp.ParamName("status"), Description: "Status: active, stable, resolved"}, {Name: mcp.ParamName("customer_impacted"), Description: "Customer impacted (true/false)"}},
	},
	{
		Name: mcp.ToolName("datadog_list_incident_attachments"), Description: "List attachments for a Datadog incident. View postmortems and linked resources attached to an outage.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("incident_id"), Description: "Incident ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_list_incident_todos"), Description: "List todos and action items for a Datadog incident. Track remediation tasks and follow-up work.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("incident_id"), Description: "Incident ID", Required: true}},
	},

	// ── Incident Services ────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_incident_services"), Description: "List Datadog incident services. View services configured for incident management and response.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_size"), Description: "Results per page (default 10)"}, {Name: mcp.ParamName("page_offset"), Description: "Pagination offset"}, {Name: mcp.ParamName("filter"), Description: "Filter services by name"}},
	},
	{
		Name: mcp.ToolName("datadog_get_incident_service"), Description: "Get details of a specific incident service. Use after list_incident_services.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("service_id"), Description: "Incident service ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_incident_service"), Description: "Create a new incident service for incident management",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Service name", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_update_incident_service"), Description: "Update an existing incident service",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("service_id"), Description: "Incident service ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New service name", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_delete_incident_service"), Description: "Delete an incident service",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("service_id"), Description: "Incident service ID", Required: true}},
	},

	// ── Incident Teams ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_incident_teams"), Description: "List Datadog incident teams. View teams configured for incident response and on-call rotation.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_size"), Description: "Results per page (default 10)"}, {Name: mcp.ParamName("page_offset"), Description: "Pagination offset"}, {Name: mcp.ParamName("filter"), Description: "Filter teams by name"}},
	},
	{
		Name: mcp.ToolName("datadog_get_incident_team"), Description: "Get details of a specific incident team. Use after list_incident_teams.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Incident team ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_incident_team"), Description: "Create a new incident team for incident response",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Team name", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_update_incident_team"), Description: "Update an existing incident team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Incident team ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New team name", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_delete_incident_team"), Description: "Delete an incident team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Incident team ID", Required: true}},
	},

	// ── Synthetics ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_synthetics_tests"), Description: "List synthetic monitoring tests",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_size"), Description: "Results per page (default 100)"}, {Name: mcp.ParamName("page_number"), Description: "Page number"}},
	},
	{
		Name: mcp.ToolName("datadog_get_synthetics_api_test"), Description: "Get a specific synthetics API test",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Test public ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_synthetics_test_result"), Description: "Get latest results for a synthetics test",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Test public ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_trigger_synthetics_tests"), Description: "Trigger synthetic tests on demand",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("public_ids"), Description: "Comma-separated test public IDs to trigger", Required: true}},
	},

	// ── Notebooks ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_notebooks"), Description: "List Datadog notebooks",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query"}, {Name: mcp.ParamName("count"), Description: "Max results (default 100)"}, {Name: mcp.ParamName("start"), Description: "Offset for pagination"}, {Name: mcp.ParamName("sort_field"), Description: "Sort by: modified or name"}, {Name: mcp.ParamName("sort_dir"), Description: "Sort direction: asc or desc"}},
	},
	{
		Name: mcp.ToolName("datadog_get_notebook"), Description: "Get a specific notebook by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Notebook ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_notebook"), Description: "Create a new notebook (JSON body)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Notebook name", Required: true}, {Name: mcp.ParamName("cells_json"), Description: "JSON array of notebook cells"}, {Name: mcp.ParamName("time_from"), Description: "Time range start"}, {Name: mcp.ParamName("time_to"), Description: "Time range end"}},
	},
	{
		Name: mcp.ToolName("datadog_delete_notebook"), Description: "Delete a notebook",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Notebook ID", Required: true}},
	},

	// ── Users ─────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_users"), Description: "List users in the organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_size"), Description: "Results per page (default 10)"}, {Name: mcp.ParamName("page_number"), Description: "Page number"}, {Name: mcp.ParamName("sort"), Description: "Sort field (e.g., name, email)"}, {Name: mcp.ParamName("filter"), Description: "Filter string"}},
	},
	{
		Name: mcp.ToolName("datadog_get_user"), Description: "Get a specific user by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "User ID", Required: true}},
	},

	// ── Teams ─────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_teams"), Description: "List Datadog teams. View organizational teams, ownership, and membership. Start here for team management.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_size"), Description: "Results per page (default 10)"}, {Name: mcp.ParamName("page_number"), Description: "Page number"}, {Name: mcp.ParamName("filter"), Description: "Filter teams by keyword/name"}, {Name: mcp.ParamName("sort"), Description: "Sort: name, -name, user_count, -user_count"}},
	},
	{
		Name: mcp.ToolName("datadog_get_team"), Description: "Get details of a specific Datadog team including handle, description, and member count. Use after list_teams.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_team"), Description: "Create a new Datadog team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Team name", Required: true}, {Name: mcp.ParamName("handle"), Description: "Team handle (URL-safe identifier)", Required: true}, {Name: mcp.ParamName("description"), Description: "Team description"}},
	},
	{
		Name: mcp.ToolName("datadog_update_team"), Description: "Update an existing team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New team name", Required: true}, {Name: mcp.ParamName("handle"), Description: "New team handle", Required: true}, {Name: mcp.ParamName("description"), Description: "New description"}},
	},
	{
		Name: mcp.ToolName("datadog_delete_team"), Description: "Delete a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_list_team_members"), Description: "List members of a Datadog team. View who belongs to a team and their roles.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("page_size"), Description: "Results per page (default 10)"}, {Name: mcp.ParamName("page_number"), Description: "Page number"}, {Name: mcp.ParamName("filter"), Description: "Filter members by keyword"}},
	},
	{
		Name: mcp.ToolName("datadog_add_team_member"), Description: "Add a user to a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("user_id"), Description: "User ID to add", Required: true}, {Name: mcp.ParamName("role"), Description: "Role: admin (omit for regular member)"}},
	},
	{
		Name: mcp.ToolName("datadog_update_team_member"), Description: "Update a team member's role",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("role"), Description: "New role: admin (omit for regular member)"}},
	},
	{
		Name: mcp.ToolName("datadog_remove_team_member"), Description: "Remove a user from a team",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("user_id"), Description: "User ID to remove", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_user_team_memberships"), Description: "Get all team memberships for a user. Find which teams a user belongs to.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_list_team_links"), Description: "List external links for a team. View runbooks, dashboards, and documentation links associated with a team.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_team_link"), Description: "Get a specific team link. Use after list_team_links.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("link_id"), Description: "Link ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_team_link"), Description: "Add a link to a team (runbook, dashboard, documentation)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("label"), Description: "Link label", Required: true}, {Name: mcp.ParamName("url"), Description: "Link URL", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_update_team_link"), Description: "Update a team link",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("link_id"), Description: "Link ID", Required: true}, {Name: mcp.ParamName("label"), Description: "New label", Required: true}, {Name: mcp.ParamName("url"), Description: "New URL", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_delete_team_link"), Description: "Delete a team link",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("link_id"), Description: "Link ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_team_permission_settings"), Description: "Get permission settings for a team. View who can manage membership and edit the team.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_update_team_permission_setting"), Description: "Update a team permission setting",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("action"), Description: "Permission action: manage_membership or edit", Required: true}, {Name: mcp.ParamName("value"), Description: "Permission value: admins, members, organization, user_access_manage, teams_manage",

		// ── Spans / APM ───────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("datadog_search_spans"), Description: "Search Datadog APM spans and distributed traces for production performance debugging. Investigate latency and service dependencies.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Span search query (e.g., 'service:web-store resource_name:GET')", Required: true}, {Name: mcp.ParamName("from"), Description: "Start time"}, {Name: mcp.ParamName("to"), Description: "End time"}, {Name: mcp.ParamName(

		// ── Software Catalog ──────────────────────────────────────────────
		"limit"), Description: "Max results (default 10)"}, {Name: mcp.ParamName("sort"), Description: "Sort: timestamp or -timestamp"}},
	},

	{
		Name: mcp.ToolName("datadog_list_services"), Description: "List services from the Datadog Software Catalog. View production microservices, dependencies, and ownership.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_size"), Description: "Results per page (default 20)"}, {Name: mcp.ParamName(

		// ── On-Call ──────────────────────────────────────────────────────
		"page_offset"), Description: "Pagination offset"}},
	},

	{
		Name: mcp.ToolName("datadog_list_oncall_schedules"), Description: "List all Datadog On-Call schedules. Start here to find schedule IDs before getting details.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("include"), Description: "Comma-separated related resources to include (e.g. teams)"}},
	},
	{
		Name: mcp.ToolName("datadog_get_oncall_schedule"), Description: "Get a Datadog On-Call schedule with layers, members, and rotation details. View who is on-call and when.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("schedule_id"), Description: "On-Call schedule ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_oncall_schedule"), Description: "Create a new On-Call schedule with layers, rotation intervals, and members. Use body_json with Datadog schedule create schema.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("body_json"), Description: `JSON body matching Datadog ScheduleCreateRequest schema: {"data":{"type":"schedules","attributes":{"name":"...","time_zone":"...","layers":[{"name":"...","effective_date":"...","rotation_start":"...","interval":{"days":7},"members":[{"user":{"id":"..."}}]}]}}}`, Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_update_oncall_schedule"), Description: "Update an existing On-Call schedule (layers, members, rotations). IMPORTANT: Always include the full relationships.teams block in body_json — omitting it silently removes the team association. Fetch current schedule with get_oncall_schedule first, then modify and submit.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("schedule_id"), Description: "Schedule ID", Required: true}, {Name: mcp.ParamName("body_json"), Description: "JSON body matching Datadog ScheduleUpdateRequest schema. MUST include relationships.teams to preserve team association.", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_delete_oncall_schedule"), Description: "Delete an On-Call schedule",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("schedule_id"), Description: "Schedule ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_schedule_oncall_user"), Description: "Get the current on-call user for a schedule. Find who is on-call right now for a given rotation.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("schedule_id"), Description: "On-Call schedule ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_list_oncall_escalation_policies"), Description: "List all Datadog On-Call escalation policies. Start here to find policy IDs before getting details.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("include"), Description: "Comma-separated related resources to include (e.g. teams,steps,steps.targets)"}},
	},
	{
		Name: mcp.ToolName("datadog_get_oncall_escalation_policy"), Description: "Get a Datadog On-Call escalation policy with steps, targets, and notification chain. View escalation rules and timing.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("policy_id"), Description: "Escalation policy ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_oncall_escalation_policy"), Description: "Create a new On-Call escalation policy with steps and targets. Use body_json with Datadog escalation policy create schema.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("body_json"), Description: `JSON body matching Datadog EscalationPolicyCreateRequest schema: {"data":{"type":"policies","attributes":{"name":"...","steps":[{"targets":[{"type":"users","id":"..."}],"escalate_after_seconds":300}]}}}`, Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_update_oncall_escalation_policy"), Description: "Update an existing On-Call escalation policy (steps, targets, timing). Fetch current policy with get_oncall_escalation_policy first, then modify and submit.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("policy_id"), Description: "Escalation policy ID", Required: true}, {Name: mcp.ParamName("body_json"), Description: "JSON body matching Datadog EscalationPolicyUpdateRequest schema", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_delete_oncall_escalation_policy"), Description: "Delete an On-Call escalation policy",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("policy_id"), Description: "Escalation policy ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_oncall_team_routing_rules"), Description: "Get On-Call routing rules for a team. View how pages are routed to schedules and escalation policies.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_set_oncall_team_routing_rules"), Description: "Set (replace) On-Call routing rules for a team. Define how pages are routed to escalation policies. Fetch current rules with get_oncall_team_routing_rules first.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}, {Name: mcp.ParamName("body_json"), Description: `JSON body matching Datadog TeamRoutingRulesRequest schema: {"data":{"type":"team_routing_rules","attributes":{"rules":[{"policy_id":"...","urgency":"high"}]}}}`, Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_team_oncall_users"), Description: "Get the current on-call users for a team. Find who is on-call right now across all team schedules.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Team ID", Required: true}},
	},

	// ── On-Call Paging ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_oncall_pages"), Description: "List On-Call pages. Filter by status and urgency to find active or past pages.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("include"), Description: "Comma-separated related resources to include"}, {Name: mcp.ParamName("status"), Description: "Filter by page status (e.g. triggered, acknowledged, resolved)"}, {Name: mcp.ParamName("urgency"), Description: "Filter by urgency (low or high)"}},
	},
	{
		Name: mcp.ToolName("datadog_get_oncall_page"), Description: "Get details of a specific On-Call page including status, responders, and timeline.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Page UUID", Required: true}, {Name: mcp.ParamName("include"), Description: "Comma-separated related resources to include"}},
	},
	{
		Name: mcp.ToolName("datadog_create_oncall_page"), Description: "Create a new On-Call page to alert responders. Page a team or user for production incidents and urgent issues.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("title"), Description: "Page title", Required: true}, {Name: mcp.ParamName("urgency"), Description: "Urgency: low or high (default high)"}, {Name: mcp.ParamName("target_id"), Description: "Target identifier (team handle, team ID, or user ID)", Required: true}, {Name: mcp.ParamName("target_type"), Description: "Target type: team_handle, team_id, or user_id (default team_handle)"}, {Name: mcp.ParamName("description"), Description: "Page description with details"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}},
	},
	{
		Name: mcp.ToolName("datadog_acknowledge_oncall_page"), Description: "Acknowledge an On-Call page to indicate responder awareness",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Page UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_escalate_oncall_page"), Description: "Escalate an On-Call page to the next responder in the escalation policy",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Page UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_resolve_oncall_page"), Description: "Resolve an On-Call page to mark the issue as handled",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Page UUID", Required: true}},
	},

	// ── Notification Channels ───────────────────────────────────────
	{
		Name: mcp.ToolName("datadog_list_user_notification_channels"), Description: "List On-Call notification channels for a user (email, Slack, SMS, push). View how a user receives on-call alerts.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_create_user_notification_channel"), Description: "Create a notification channel for a user (email, Slack, SMS, push).",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("body_json"), Description: "JSON body matching Datadog CreateUserNotificationChannelRequest schema", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_user_notification_channel"), Description: "Get a specific notification channel for a user.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("channel_id"), Description: "Notification channel ID", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_delete_user_notification_channel"), Description: "Delete a notification channel for a user.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("channel_id"), Description: "Notification channel ID",

		// ── Notification Rules ─────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("datadog_list_user_notification_rules"), Description: "List On-Call notification rules for a user. Rules define when and how a user is notified for on-call pages.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("include"), Description: "Comma-separated related resources to include"}},
	},
	{
		Name: mcp.ToolName("datadog_create_user_notification_rule"), Description: "Create a notification rule for a user defining when and how they are notified for on-call pages.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("body_json"), Description: "JSON body matching Datadog CreateOnCallNotificationRuleRequest schema", Required: true}},
	},
	{
		Name: mcp.ToolName("datadog_get_user_notification_rule"), Description: "Get a specific notification rule for a user.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("rule_id"), Description: "Notification rule ID", Required: true}, {Name: mcp.ParamName("include"), Description: "Comma-separated related resources to include"}},
	},
	{
		Name: mcp.ToolName("datadog_update_user_notification_rule"), Description: "Update a notification rule for a user.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("rule_id"), Description: "Notification rule ID", Required: true}, {Name: mcp.ParamName("body_json"), Description: "JSON body matching Datadog UpdateOnCallNotificationRuleRequest schema", Required: true}, {Name: mcp.ParamName("include"), Description: "Comma-separated related resources to include"}},
	},
	{
		Name: mcp.ToolName("datadog_delete_user_notification_rule"), Description: "Delete a notification rule for a user.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID", Required: true}, {Name: mcp.ParamName("rule_id"), Description: "Notification rule ID",

		// ── IP Ranges ─────────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("datadog_get_ip_ranges"), Description: "Get Datadog IP address ranges used for inbound/outbound traffic",
		Parameters: []mcp.Parameter{},
	},
}
