package datadog

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Logs ──────────────────────────────────────────────────────────
	{
		Name: "datadog_search_logs", Description: "Search Datadog logs with a query string",
		Parameters: map[string]string{"query": "Log search query (e.g., 'service:nginx status:error')", "from": "Start time (ISO 8601, epoch seconds, or relative like 'now-1h')", "to": "End time (default: now)", "limit": "Max results (default 50, max 1000)", "sort": "Sort order: timestamp (asc) or -timestamp (desc, default)"},
		Required:   []string{"query"},
	},
	{
		Name: "datadog_aggregate_logs", Description: "Aggregate logs using compute operations (count, sum, avg, etc.)",
		Parameters: map[string]string{"query": "Log search query", "compute_type": "Aggregation type: count, cardinality, avg, sum, min, max, percentile", "compute_field": "Field to aggregate on (required for non-count types, e.g., @duration)", "group_by": "Field to group by (e.g., service, @http.status_code)", "from": "Start time", "to": "End time"},
		Required:   []string{"query", "compute_type"},
	},

	// ── Metrics ───────────────────────────────────────────────────────
	{
		Name: "datadog_query_metrics", Description: "Query Datadog metrics timeseries data",
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
		Name: "datadog_list_monitors", Description: "List Datadog monitors with optional filters",
		Parameters: map[string]string{"query": "Filter query (e.g., 'status:alert tag:env:prod')", "page": "Page number (0-based)", "page_size": "Results per page (default 100)"},
	},
	{
		Name: "datadog_search_monitors", Description: "Search monitors by query string",
		Parameters: map[string]string{"query": "Search query (e.g., 'type:metric status:alert')", "page": "Page number", "per_page": "Results per page (default 30)"},
	},
	{
		Name: "datadog_get_monitor", Description: "Get a specific monitor by ID",
		Parameters: map[string]string{"id": "Monitor ID"},
		Required:   []string{"id"},
	},
	{
		Name: "datadog_create_monitor", Description: "Create a new monitor",
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
		Name: "datadog_list_dashboards", Description: "List all dashboards",
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
		Name: "datadog_list_events", Description: "List events with optional filters",
		Parameters: map[string]string{"query": "Event search query", "from": "Start time", "to": "End time", "limit": "Max results (default 10)", "sort": "Sort: timestamp (asc) or -timestamp (desc)"},
	},
	{
		Name: "datadog_search_events", Description: "Search events with a query",
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
		Name: "datadog_list_hosts", Description: "List hosts with optional filters",
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
		Name: "datadog_list_incidents", Description: "List incidents",
		Parameters: map[string]string{"page_size": "Results per page (default 10)", "page_offset": "Pagination offset"},
	},
	{
		Name: "datadog_get_incident", Description: "Get a specific incident by ID",
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

	// ── Spans / APM ───────────────────────────────────────────────────
	{
		Name: "datadog_search_spans", Description: "Search APM spans/traces",
		Parameters: map[string]string{"query": "Span search query (e.g., 'service:web-store resource_name:GET')", "from": "Start time", "to": "End time", "limit": "Max results (default 10)", "sort": "Sort: timestamp or -timestamp"},
		Required:   []string{"query"},
	},

	// ── Software Catalog ──────────────────────────────────────────────
	{
		Name: "datadog_list_services", Description: "List services from the Datadog Software Catalog",
		Parameters: map[string]string{"page_size": "Results per page (default 20)", "page_offset": "Pagination offset"},
	},

	// ── IP Ranges ─────────────────────────────────────────────────────
	{
		Name: "datadog_get_ip_ranges", Description: "Get Datadog IP address ranges used for inbound/outbound traffic",
		Parameters: map[string]string{},
	},
}
