package datadog

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Logs ──────────────────────────────────────────────────────────
	"datadog_search_logs":    {"status", "service", "host", "message", "timestamp", "tags"},
	"datadog_aggregate_logs": {"buckets", "meta"},

	// ── Metrics ──────────────────────────────────────────────────────
	"datadog_query_metrics":       {"series[].metric", "series[].pointlist", "series[].scope", "series[].unit"},
	"datadog_list_active_metrics": {"metrics", "from"},
	"datadog_search_metrics":      {"metrics"},
	"datadog_get_metric_metadata": {"type", "description", "short_name", "unit", "per_unit", "integration"},

	// ── Monitors ─────────────────────────────────────────────────────
	"datadog_list_monitors":   {"id", "name", "type", "query", "overall_state", "message", "tags", "created", "modified"},
	"datadog_search_monitors": {"monitors[].id", "monitors[].name", "monitors[].type", "monitors[].overall_state", "monitors[].query", "monitors[].tags", "metadata"},
	"datadog_get_monitor":     {"id", "name", "type", "query", "overall_state", "message", "tags", "options", "created", "modified"},

	// ── Dashboards ───────────────────────────────────────────────────
	"datadog_list_dashboards": {"dashboards[].id", "dashboards[].title", "dashboards[].description", "dashboards[].author_handle", "dashboards[].layout_type", "dashboards[].url", "dashboards[].created_at", "dashboards[].modified_at"},
	"datadog_get_dashboard":   {"id", "title", "description", "author_handle", "layout_type", "widgets", "template_variables", "url", "created_at", "modified_at"},

	// ── Events ───────────────────────────────────────────────────────
	"datadog_list_events":   {"events[].id", "events[].title", "events[].text", "events[].date_happened", "events[].source", "events[].tags", "events[].priority", "events[].alert_type"},
	"datadog_search_events": {"events[].id", "events[].title", "events[].text", "events[].date_happened", "events[].source", "events[].tags", "events[].priority"},
	"datadog_get_event":     {"id", "title", "text", "date_happened", "source", "tags", "priority", "alert_type"},

	// ── Hosts & Tags ─────────────────────────────────────────────────
	"datadog_list_hosts":      {"host_list[].name", "host_list[].up", "host_list[].meta.platform", "host_list[].apps", "host_list[].tags_by_source", "host_list[].last_reported_time", "total_matching", "total_returned"},
	"datadog_get_host_totals": {"total_active", "total_up"},
	"datadog_list_tags":       {"tags"},
	"datadog_get_host_tags":   {"tags"},

	// ── SLOs ─────────────────────────────────────────────────────────
	"datadog_list_slos":       {"data[].id", "data[].name", "data[].type", "data[].description", "data[].thresholds", "data[].tags", "data[].overall_status"},
	"datadog_search_slos":     {"data.attributes[].id", "data.attributes[].name", "data.attributes[].status", "data.attributes[].overall_status"},
	"datadog_get_slo":         {"data.id", "data.name", "data.type", "data.description", "data.thresholds", "data.tags", "data.overall_status"},
	"datadog_get_slo_history": {"data.overall.sli_value", "data.overall.span_precision", "data.thresholds"},

	// ── Downtimes ────────────────────────────────────────────────────
	"datadog_list_downtimes": {"id", "scope", "message", "start", "end", "monitor_id", "active", "disabled"},
	"datadog_get_downtime":   {"id", "scope", "message", "start", "end", "monitor_id", "active", "disabled", "recurrence"},

	// ── Incidents ────────────────────────────────────────────────────
	"datadog_list_incidents": {"data[].id", "data[].attributes.title", "data[].attributes.severity", "data[].attributes.status", "data[].attributes.created", "data[].attributes.modified"},
	"datadog_get_incident":   {"data.id", "data.attributes.title", "data.attributes.severity", "data.attributes.status", "data.attributes.fields", "data.attributes.created", "data.attributes.modified"},

	// ── Synthetics ───────────────────────────────────────────────────
	"datadog_list_synthetics_tests":      {"tests[].public_id", "tests[].name", "tests[].type", "tests[].status", "tests[].tags", "tests[].locations"},
	"datadog_get_synthetics_api_test":    {"public_id", "name", "type", "status", "tags", "config", "locations", "message"},
	"datadog_get_synthetics_test_result": {"result_id", "status", "check_time", "result"},

	// ── Notebooks ────────────────────────────────────────────────────
	"datadog_list_notebooks": {"data[].id", "data[].attributes.name", "data[].attributes.author.handle", "data[].attributes.status", "data[].attributes.created", "data[].attributes.modified"},
	"datadog_get_notebook":   {"data.id", "data.attributes.name", "data.attributes.cells", "data.attributes.author.handle", "data.attributes.status", "data.attributes.created", "data.attributes.modified"},

	// ── Users ────────────────────────────────────────────────────────
	"datadog_list_users": {"data[].id", "data[].attributes.name", "data[].attributes.handle", "data[].attributes.email", "data[].attributes.status", "data[].attributes.disabled"},
	"datadog_get_user":   {"data.id", "data.attributes.name", "data.attributes.handle", "data.attributes.email", "data.attributes.status", "data.attributes.disabled"},

	// ── APM ──────────────────────────────────────────────────────────
	"datadog_search_spans":  {"data[].attributes.service", "data[].attributes.resource_name", "data[].attributes.status", "data[].attributes.duration", "data[].attributes.start", "data[].attributes.span_id", "data[].attributes.trace_id"},
	"datadog_list_services": {"data[].id", "data[].attributes.schema.dd-service", "data[].attributes.schema.team", "data[].attributes.schema.description"},

	// ── IP Ranges ────────────────────────────────────────────────────
	"datadog_get_ip_ranges": {"agents", "api", "logs", "webhooks", "synthetics"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("datadog: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
