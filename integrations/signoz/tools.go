package signoz

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Services ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_services"), Description: "List all application services and their metrics (latency, error rate, throughput). Start here for APM, service health monitoring, and performance debugging.",
		Parameters: map[string]string{"start": "Start time in epoch milliseconds", "end": "End time in epoch milliseconds"},
		Required:   []string{"start", "end"},
	},
	{
		Name: mcp.ToolName("signoz_get_service_overview"), Description: "Get detailed overview metrics for a specific service over a time range. Use after list_services for latency percentiles, error rates, and request counts.",
		Parameters: map[string]string{"service": "Service name", "start": "Start time in epoch milliseconds", "end": "End time in epoch milliseconds", "step": "Step interval in seconds (default: 60)"},
		Required:   []string{"service", "start", "end"},
	},
	{
		Name: mcp.ToolName("signoz_top_operations"), Description: "Get top operations for a service ranked by latency, error rate, or call count. Use after list_services to drill into hotspot endpoints and slow operations.",
		Parameters: map[string]string{"service": "Service name", "start": "Start time in epoch milliseconds", "end": "End time in epoch milliseconds"},
		Required:   []string{"service", "start", "end"},
	},
	{
		Name: mcp.ToolName("signoz_top_level_operations"), Description: "Get top-level (entry point) operations across all services. Useful for finding the most called API endpoints and root spans.",
		Parameters: map[string]string{"start": "Start time in epoch milliseconds", "end": "End time in epoch milliseconds"},
		Required:   []string{"start", "end"},
	},
	{
		Name: mcp.ToolName("signoz_entry_point_operations"), Description: "Get entry point operations for a service (v2). Returns the first spans in a trace for each service.",
		Parameters: map[string]string{"service": "Service name", "start": "Start time in epoch milliseconds", "end": "End time in epoch milliseconds"},
		Required:   []string{"service", "start", "end"},
	},

	// ── Query: Logs ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_search_logs"), Description: "Search and filter log entries. Supports attribute filters (severity, service, body text), ordering, and pagination. Start here for log exploration, debugging, and error investigation.",
		Parameters: map[string]string{
			"start":  "Start time in epoch milliseconds",
			"end":    "End time in epoch milliseconds",
			"filter": "Filter expression, e.g. \"severity_text = 'ERROR'\" or \"body CONTAINS 'timeout'\"",
			"limit":  "Max logs to return (default: 20, max: 100)",
			"offset": "Offset for pagination (default: 0)",
		},
		Required: []string{"start", "end"},
	},

	// ── Query: Traces ────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_search_traces"), Description: "Search distributed traces with filters. Find slow requests, errors, and specific operations across services. Start here for trace exploration and latency debugging.",
		Parameters: map[string]string{
			"start":   "Start time in epoch milliseconds",
			"end":     "End time in epoch milliseconds",
			"service": "Filter by service name",
			"filter":  "Filter expression, e.g. \"hasError = true\" or \"durationNano > 1000000000\"",
			"limit":   "Max traces to return (default: 20, max: 100)",
			"offset":  "Offset for pagination (default: 0)",
		},
		Required: []string{"start", "end"},
	},
	{
		Name: mcp.ToolName("signoz_get_trace"), Description: "Get all spans for a specific trace by its trace ID. Shows the full request path across services with timing breakdown. Use after search_traces.",
		Parameters: map[string]string{"trace_id": "Trace ID"},
		Required:   []string{"trace_id"},
	},

	// ── Query: Metrics ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_query_metrics"), Description: "Query time-series metrics data. Supports aggregation (avg, sum, min, max, count, rate, p50-p99), filtering, and group-by. Start here for metrics exploration, infrastructure monitoring, and custom dashboards.",
		Parameters: map[string]string{
			"start":        "Start time in epoch milliseconds",
			"end":          "End time in epoch milliseconds",
			"metric_name":  "Metric name, e.g. 'signoz_calls_total' or 'system.cpu.load_average.1m'",
			"aggregate_op": "Aggregation: avg, sum, min, max, count, rate, p50, p75, p90, p95, p99 (default: avg)",
			"filter":       "Filter expression, e.g. \"service_name = 'frontend'\"",
			"group_by":     "Comma-separated attribute keys to group by, e.g. 'service_name,host'",
			"step":         "Step interval in seconds (default: 60)",
		},
		Required: []string{"start", "end", "metric_name"},
	},

	// ── Dashboards ───────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_dashboards"), Description: "List all dashboards. Start here for dashboard discovery and visualization management.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("signoz_get_dashboard"), Description: "Get a specific dashboard by ID including all panels and widget configurations. Use after list_dashboards.",
		Parameters: map[string]string{"id": "Dashboard ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("signoz_create_dashboard"), Description: "Create a new dashboard with title, description, and optional tags.",
		Parameters: map[string]string{"title": "Dashboard title", "description": "Dashboard description", "tags": "Comma-separated tags for categorization"},
		Required:   []string{"title"},
	},
	{
		Name: mcp.ToolName("signoz_update_dashboard"), Description: "Update an existing dashboard. Send the full dashboard JSON object (get it first with get_dashboard).",
		Parameters: map[string]string{"id": "Dashboard ID", "dashboard": "Full dashboard JSON object to replace"},
		Required:   []string{"id", "dashboard"},
	},
	{
		Name: mcp.ToolName("signoz_delete_dashboard"), Description: "Delete a dashboard by ID.",
		Parameters: map[string]string{"id": "Dashboard ID"},
		Required:   []string{"id"},
	},

	// ── Alerts (Rules) ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_alerts"), Description: "List all alert rules including their current state (firing, inactive, pending). Start here for alert management and on-call monitoring.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("signoz_get_alert"), Description: "Get a specific alert rule by ID with full configuration and current state. Use after list_alerts.",
		Parameters: map[string]string{"id": "Alert rule ID"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("signoz_create_alert"), Description: "Create a new alert rule. Requires a full alert rule definition JSON body.",
		Parameters: map[string]string{"rule": "Full alert rule definition JSON object"},
		Required:   []string{"rule"},
	},
	{
		Name: mcp.ToolName("signoz_update_alert"), Description: "Update an existing alert rule. Send the full rule object (get it first with get_alert).",
		Parameters: map[string]string{"id": "Alert rule ID", "rule": "Full alert rule definition JSON object"},
		Required:   []string{"id", "rule"},
	},
	{
		Name: mcp.ToolName("signoz_delete_alert"), Description: "Delete an alert rule by ID.",
		Parameters: map[string]string{"id": "Alert rule ID"},
		Required:   []string{"id"},
	},

	// ── Saved Views ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_saved_views"), Description: "List all saved explorer views for logs and traces. Use to find pre-configured query views.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("signoz_get_saved_view"), Description: "Get a specific saved view by ID with full query configuration. Use after list_saved_views.",
		Parameters: map[string]string{"view_id": "Saved view ID"},
		Required:   []string{"view_id"},
	},
	{
		Name: mcp.ToolName("signoz_create_saved_view"), Description: "Create a new saved explorer view with query configuration.",
		Parameters: map[string]string{"view": "Saved view definition JSON object"},
		Required:   []string{"view"},
	},
	{
		Name: mcp.ToolName("signoz_update_saved_view"), Description: "Update an existing saved view.",
		Parameters: map[string]string{"view_id": "Saved view ID", "view": "Updated saved view definition JSON object"},
		Required:   []string{"view_id", "view"},
	},
	{
		Name: mcp.ToolName("signoz_delete_saved_view"), Description: "Delete a saved view by ID.",
		Parameters: map[string]string{"view_id": "Saved view ID"},
		Required:   []string{"view_id"},
	},

	// ── Notification Channels ────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_channels"), Description: "List all configured notification channels (Slack, email, PagerDuty, webhooks) for alerts.",
		Parameters: map[string]string{},
	},

	// ── Extras ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_get_version"), Description: "Get SigNoz server version, edition (community/enterprise), and setup status.",
		Parameters: map[string]string{},
	},
}
