package signoz

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// Services (unwrapped from {"status":"success","data":[...]})
	mcp.ToolName("signoz_list_services"):          {"serviceName", "p99", "avgDuration", "numCalls", "callRate", "numErrors", "errorRate"},
	mcp.ToolName("signoz_get_service_overview"):   {"result[].queryName", "result[].series[].labels", "result[].series[].values"},
	mcp.ToolName("signoz_top_operations"):         {"name", "p50", "p90", "p99", "numCalls", "errorRate"},
	mcp.ToolName("signoz_top_level_operations"):   {"name", "serviceName", "numCalls"},
	mcp.ToolName("signoz_entry_point_operations"): {"name", "serviceName", "numCalls"},

	// Query (unwrapped from {"status":"success","data":{...}})
	mcp.ToolName("signoz_search_logs"):   {"result[].queryName", "result[].list[].timestamp", "result[].list[].data.severity_text", "result[].list[].data.body", "result[].list[].data.resources_string"},
	mcp.ToolName("signoz_search_traces"): {"result[].queryName", "result[].list[].timestamp", "result[].list[].data.traceID", "result[].list[].data.spanID", "result[].list[].data.serviceName", "result[].list[].data.name", "result[].list[].data.durationNano", "result[].list[].data.hasError"},
	mcp.ToolName("signoz_get_trace"):     {"columns", "events[].SpanId", "events[].TraceId", "events[].ServiceName", "events[].Name", "events[].DurationNano", "events[].HasError", "events[].StatusCodeString"},
	mcp.ToolName("signoz_query_metrics"): {"result[].queryName", "result[].series[].labels", "result[].series[].values"},

	// Dashboards (unwrapped: array of {id, data.data.title, createdAt, ...})
	mcp.ToolName("signoz_list_dashboards"): {"id", "data.data.title", "data.data.description", "data.data.tags", "createdAt", "updatedAt"},
	mcp.ToolName("signoz_get_dashboard"):   {"id", "data.data.title", "data.data.description", "data.data.tags", "data.data.layout", "data.data.widgets[].id", "data.data.widgets[].title", "data.data.widgets[].panelTypes", "createdAt"},

	// Alerts (unwrapped: {rules: [...]})
	mcp.ToolName("signoz_list_alerts"): {"rules[].id", "rules[].state", "rules[].alert", "rules[].alertType", "rules[].labels", "rules[].evalTime"},
	mcp.ToolName("signoz_get_alert"):   {"id", "state", "alert", "alertType", "condition", "labels", "annotations", "evalTime"},

	// Saved Views (unwrapped: array or null)
	mcp.ToolName("signoz_list_saved_views"): {"uuid", "name", "category", "createdAt", "updatedAt"},
	mcp.ToolName("signoz_get_saved_view"):   {"uuid", "name", "category", "compositeQuery", "createdAt", "updatedAt"},

	// Notification Channels (unwrapped: array)
	mcp.ToolName("signoz_list_channels"): {"id", "name", "type", "createdAt"},

	// Extras (no envelope)
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
