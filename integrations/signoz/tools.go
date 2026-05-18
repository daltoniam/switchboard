package signoz

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Services ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_services"), Description: "List all application services and their metrics (latency, error rate, throughput). Start here for APM, service health monitoring, and performance debugging.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("end"), Description: "End time in epoch milliseconds", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_get_service_overview"), Description: "Get detailed overview metrics for a specific service over a time range. Use after list_services for latency percentiles, error rates, and request counts.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("service"), Description: "Service name", Required: true}, {Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("end"), Description: "End time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("step"), Description: "Step interval in seconds (default: 60)"}},
	},
	{
		Name: mcp.ToolName("signoz_top_operations"), Description: "Get top operations for a service ranked by latency, error rate, or call count. Use after list_services to drill into hotspot endpoints and slow operations.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("service"), Description: "Service name", Required: true}, {Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("end"), Description: "End time in epoch milliseconds", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_top_level_operations"), Description: "Get top-level (entry point) operations across all services. Useful for finding the most called API endpoints and root spans.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("end"), Description: "End time in epoch milliseconds", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_entry_point_operations"), Description: "Get entry point operations for a service (v2). Returns the first spans in a trace for each service.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("service"), Description: "Service name", Required: true}, {Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName(

		// ── Query: Logs ──────────────────────────────────────────────────
		"end"), Description: "End time in epoch milliseconds", Required: true}},
	},

	{
		Name: mcp.ToolName("signoz_search_logs"), Description: "Search and filter log entries. Supports attribute filters (severity, service, body text), ordering, and pagination. Start here for log exploration, debugging, and error investigation.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("end"), Description: "End time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("filter"), Description: `Single filter expression: key op value, e.g. "severity_text = 'ERROR'". Only one expression per call.`}, {Name: mcp.ParamName("limit"), Description: "Max logs to return (default: 20, max: 100)"},

		// ── Query: Traces ────────────────────────────────────────────────
		{Name: mcp.ParamName("offset"), Description: "Offset for pagination (default: 0)"}},
	},

	{
		Name: mcp.ToolName("signoz_search_traces"), Description: "Search distributed traces with filters. Find slow requests, errors, and specific operations across services. Start here for trace exploration and latency debugging.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("end"), Description: "End time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("service"), Description: "Filter by service name"}, {Name: mcp.ParamName("filter"), Description: `Single filter expression: key op value, e.g. "hasError = true". Only one expression per call; use service param for service filtering.`}, {Name: mcp.ParamName("limit"), Description: "Max traces to return (default: 20, max: 100)"}, {Name: mcp.ParamName("offset"), Description: "Offset for pagination (default: 0)"}},
	},
	{
		Name: mcp.ToolName("signoz_get_trace"), Description: "Get all spans for a specific trace by its trace ID. Shows the full request path across services with timing breakdown. Use after search_traces.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("trace_id"), Description: "Trace ID", Required: true}},
	},

	// ── Query: Metrics ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_query_metrics"), Description: "Query time-series metrics data. Supports aggregation (avg, sum, min, max, count, rate, p50-p99), filtering, and group-by. Start here for metrics exploration, infrastructure monitoring, and custom dashboards.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("start"), Description: "Start time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("end"), Description: "End time in epoch milliseconds", Required: true}, {Name: mcp.ParamName("metric_name"), Description: "Metric name, e.g. 'signoz_calls_total' or 'system.cpu.load_average.1m'", Required: true}, {Name: mcp.ParamName("aggregate_op"), Description: "Aggregation: avg, sum, min, max, count, rate, p50, p75, p90, p95, p99 (default: avg)"}, {Name: mcp.ParamName("filter"), Description: `Single filter expression: key op value, e.g. "service_name = 'frontend'". Only one expression per call.`}, {Name: mcp.ParamName("group_by"), Description: "Comma-separated attribute keys to group by, e.g. 'service_name,host'"},

		// ── Dashboards ───────────────────────────────────────────────────
		{Name: mcp.ParamName("step"), Description: "Step interval in seconds (default: 60)"}},
	},

	{
		Name: mcp.ToolName("signoz_list_dashboards"), Description: "List all dashboards. Start here for dashboard discovery and visualization management.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("signoz_get_dashboard"), Description: "Get a specific dashboard by ID including all panels and widget configurations. Use after list_dashboards.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Dashboard ID", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_create_dashboard"), Description: "Create a new dashboard with title, description, and optional tags.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("title"), Description: "Dashboard title", Required: true}, {Name: mcp.ParamName("description"), Description: "Dashboard description"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags for categorization"}},
	},
	{
		Name: mcp.ToolName("signoz_update_dashboard"), Description: "Update an existing dashboard. Send dashboard content (title, widgets, layout, tags) or the full get_dashboard response — nesting is auto-corrected. The API wraps content automatically.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Dashboard ID", Required: true}, {Name: mcp.ParamName("dashboard"), Description: "Dashboard content object with title, widgets, layout, tags, variables", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_delete_dashboard"), Description: "Delete a dashboard by ID.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Dashboard ID", Required: true}},
	},

	// ── Alerts (Rules) ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_alerts"), Description: "List all alert rules including their current state (firing, inactive, pending). Start here for alert management and on-call monitoring.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("signoz_get_alert"), Description: "Get a specific alert rule by ID with full configuration and current state. Use after list_alerts.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Alert rule ID", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_create_alert"), Description: "Create a new alert rule. Requires a full alert rule definition JSON body.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("rule"), Description: "Full alert rule definition JSON object", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_update_alert"), Description: "Update an existing alert rule. Send the full rule object (get it first with get_alert).",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Alert rule ID", Required: true}, {Name: mcp.ParamName("rule"), Description: "Full alert rule definition JSON object", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_delete_alert"), Description: "Delete an alert rule by ID.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Alert rule ID", Required: true}},
	},

	// ── Saved Views ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_saved_views"), Description: "List all saved explorer views for logs and traces. Use to find pre-configured query views.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("signoz_get_saved_view"), Description: "Get a specific saved view by ID with full query configuration. Use after list_saved_views.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("view_id"), Description: "Saved view ID", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_create_saved_view"), Description: "Create a new saved explorer view with query configuration.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("view"), Description: "Saved view definition JSON object", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_update_saved_view"), Description: "Update an existing saved view.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("view_id"), Description: "Saved view ID", Required: true}, {Name: mcp.ParamName("view"), Description: "Updated saved view definition JSON object", Required: true}},
	},
	{
		Name: mcp.ToolName("signoz_delete_saved_view"), Description: "Delete a saved view by ID.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("view_id"), Description: "Saved view ID", Required: true}},
	},

	// ── Notification Channels ────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_list_channels"), Description: "List all configured notification channels (Slack, email, PagerDuty, webhooks) for alerts.",
		Parameters: []mcp.Parameter{},
	},

	// ── Extras ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("signoz_get_version"), Description: "Get SigNoz server version, edition (community/enterprise), and setup status.",
		Parameters: []mcp.Parameter{},
	},
}
