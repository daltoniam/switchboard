package signoz

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// Services
	mcp.ToolName("signoz_list_services"):          {"serviceName", "p99", "avgDuration", "numCalls", "callRate", "numErrors", "errorRate"},
	mcp.ToolName("signoz_get_service_overview"):   {"queryName", "series[].labels", "series[].values"},
	mcp.ToolName("signoz_top_operations"):         {"name", "p50", "p90", "p99", "numCalls", "errorRate"},
	mcp.ToolName("signoz_top_level_operations"):   {"name", "serviceName", "numCalls"},
	mcp.ToolName("signoz_entry_point_operations"): {"name", "serviceName", "numCalls"},

	// Query
	mcp.ToolName("signoz_search_logs"):   {"queryName", "list[].timestamp", "list[].severity_text", "list[].body", "list[].resources_string"},
	mcp.ToolName("signoz_search_traces"): {"queryName", "list[].timestamp", "list[].data.traceID", "list[].data.spanID", "list[].data.serviceName", "list[].data.name", "list[].data.durationNano", "list[].data.hasError"},
	mcp.ToolName("signoz_get_trace"):     {"columns", "events[].SpanId", "events[].TraceId", "events[].ServiceName", "events[].Name", "events[].DurationNano", "events[].HasError", "events[].StatusCodeString"},
	mcp.ToolName("signoz_query_metrics"): {"queryName", "series[].labels", "series[].values"},

	// Dashboards
	mcp.ToolName("signoz_list_dashboards"): {"id", "uuid", "title", "description", "tags", "createdAt", "updatedAt"},
	mcp.ToolName("signoz_get_dashboard"):   {"id", "uuid", "title", "description", "tags", "layout", "widgets[].id", "widgets[].title", "widgets[].panelTypes"},

	// Alerts
	mcp.ToolName("signoz_list_alerts"): {"id", "state", "alert", "alertType", "labels", "annotations", "evalTime", "lastEvaluation"},
	mcp.ToolName("signoz_get_alert"):   {"id", "state", "alert", "alertType", "condition", "labels", "annotations", "evalTime", "lastEvaluation"},

	// Saved Views
	mcp.ToolName("signoz_list_saved_views"): {"uuid", "name", "category", "createdAt", "updatedAt"},
	mcp.ToolName("signoz_get_saved_view"):   {"uuid", "name", "category", "compositeQuery", "createdAt", "updatedAt"},

	// Notification Channels
	mcp.ToolName("signoz_list_channels"): {"id", "name", "type", "createdAt"},

	// Extras
	mcp.ToolName("signoz_get_version"): {"version", "ee", "setupCompleted"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("signoz: bad compact spec for %s: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
