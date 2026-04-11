package datadog

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Logs ──────────────────────────────────────────────────────────
	mcp.ToolName("datadog_search_logs"):    {"status", "service", "host", "message", "timestamp", "tags"},
	mcp.ToolName("datadog_aggregate_logs"): {"buckets", "meta"},

	// ── Metrics ──────────────────────────────────────────────────────
	mcp.ToolName("datadog_query_metrics"):       {"series[].metric", "series[].pointlist", "series[].scope", "series[].unit"},
	mcp.ToolName("datadog_list_active_metrics"): {"metrics", "from"},
	mcp.ToolName("datadog_search_metrics"):      {"metrics"},
	mcp.ToolName("datadog_get_metric_metadata"): {"type", "description", "short_name", "unit", "per_unit", "integration"},

	// ── Monitors ─────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_monitors"):   {"id", "name", "type", "query", "overall_state", "message", "tags", "created", "modified"},
	mcp.ToolName("datadog_search_monitors"): {"monitors[].id", "monitors[].name", "monitors[].type", "monitors[].overall_state", "monitors[].query", "monitors[].tags", "metadata"},
	mcp.ToolName("datadog_get_monitor"):     {"id", "name", "type", "query", "overall_state", "message", "tags", "options", "created", "modified"},

	// ── Dashboards ───────────────────────────────────────────────────
	mcp.ToolName("datadog_list_dashboards"): {"dashboards[].id", "dashboards[].title", "dashboards[].description", "dashboards[].author_handle", "dashboards[].layout_type", "dashboards[].url", "dashboards[].created_at", "dashboards[].modified_at"},
	mcp.ToolName("datadog_get_dashboard"):   {"id", "title", "description", "author_handle", "layout_type", "widgets", "template_variables", "url", "created_at", "modified_at"},

	// ── Events ───────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_events"):   {"events[].id", "events[].title", "events[].text", "events[].date_happened", "events[].source", "events[].tags", "events[].priority", "events[].alert_type"},
	mcp.ToolName("datadog_search_events"): {"events[].id", "events[].title", "events[].text", "events[].date_happened", "events[].source", "events[].tags", "events[].priority"},
	mcp.ToolName("datadog_get_event"):     {"id", "title", "text", "date_happened", "source", "tags", "priority", "alert_type"},

	// ── Hosts & Tags ─────────────────────────────────────────────────
	mcp.ToolName("datadog_list_hosts"):      {"host_list[].name", "host_list[].up", "host_list[].meta.platform", "host_list[].apps", "host_list[].tags_by_source", "host_list[].last_reported_time", "total_matching", "total_returned"},
	mcp.ToolName("datadog_get_host_totals"): {"total_active", "total_up"},
	mcp.ToolName("datadog_list_tags"):       {"tags"},
	mcp.ToolName("datadog_get_host_tags"):   {"tags"},

	// ── SLOs ─────────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_slos"):       {"data[].id", "data[].name", "data[].type", "data[].description", "data[].thresholds", "data[].tags", "data[].overall_status"},
	mcp.ToolName("datadog_search_slos"):     {"data.attributes[].id", "data.attributes[].name", "data.attributes[].status", "data.attributes[].overall_status"},
	mcp.ToolName("datadog_get_slo"):         {"data.id", "data.name", "data.type", "data.description", "data.thresholds", "data.tags", "data.overall_status"},
	mcp.ToolName("datadog_get_slo_history"): {"data.overall.sli_value", "data.overall.span_precision", "data.thresholds"},

	// ── Downtimes ────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_downtimes"): {"id", "scope", "message", "start", "end", "monitor_id", "active", "disabled"},
	mcp.ToolName("datadog_get_downtime"):   {"id", "scope", "message", "start", "end", "monitor_id", "active", "disabled", "recurrence"},

	// ── Incidents ────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_incidents"):            {"data[].id", "data[].attributes.title", "data[].attributes.severity", "data[].attributes.status", "data[].attributes.created", "data[].attributes.modified"},
	mcp.ToolName("datadog_search_incidents"):          {"data[].id", "data[].attributes.title", "data[].attributes.severity", "data[].attributes.status", "data[].attributes.created", "data[].attributes.modified"},
	mcp.ToolName("datadog_get_incident"):              {"data.id", "data.attributes.title", "data.attributes.severity", "data.attributes.status", "data.attributes.fields", "data.attributes.created", "data.attributes.modified"},
	mcp.ToolName("datadog_list_incident_attachments"): {"data[].id", "data[].type", "data[].attributes.attachment_type", "data[].attributes.modified"},
	mcp.ToolName("datadog_list_incident_todos"):       {"data[].id", "data[].attributes.content", "data[].attributes.completed", "data[].attributes.due_date", "data[].attributes.created", "data[].attributes.modified"},

	// ── Incident Services ────────────────────────────────────────────
	mcp.ToolName("datadog_list_incident_services"): {"data[].id", "data[].attributes.name", "data[].attributes.created", "data[].attributes.modified"},
	mcp.ToolName("datadog_get_incident_service"):   {"data.id", "data.attributes.name", "data.attributes.created", "data.attributes.modified"},

	// ── Incident Teams ───────────────────────────────────────────────
	mcp.ToolName("datadog_list_incident_teams"): {"data[].id", "data[].attributes.name", "data[].attributes.created", "data[].attributes.modified"},
	mcp.ToolName("datadog_get_incident_team"):   {"data.id", "data.attributes.name", "data.attributes.created", "data.attributes.modified"},

	// ── On-Call ──────────────────────────────────────────────────────
	mcp.ToolName("datadog_get_oncall_schedule"):           {"data.id", "data.attributes.name", "data.attributes.time_zone", "data.relationships"},
	mcp.ToolName("datadog_get_schedule_oncall_user"):      {"data.id", "data.attributes", "data.relationships"},
	mcp.ToolName("datadog_get_oncall_escalation_policy"):  {"data.id", "data.attributes.name", "data.relationships"},
	mcp.ToolName("datadog_get_oncall_team_routing_rules"): {"data.id", "data.attributes", "data.relationships"},
	mcp.ToolName("datadog_get_team_oncall_users"):         {"data.id", "data.relationships"},

	// ── Teams ────────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_teams"):                   {"data[].id", "data[].attributes.name", "data[].attributes.handle", "data[].attributes.description", "data[].attributes.user_count", "data[].attributes.link_count"},
	mcp.ToolName("datadog_get_team"):                     {"data.id", "data.attributes.name", "data.attributes.handle", "data.attributes.description", "data.attributes.user_count", "data.attributes.link_count", "data.attributes.created_at", "data.attributes.modified_at"},
	mcp.ToolName("datadog_list_team_members"):            {"data[].id", "data[].attributes.role", "data[].relationships.user.data.id"},
	mcp.ToolName("datadog_get_user_team_memberships"):    {"data[].id", "data[].attributes.role", "data[].relationships.team.data.id"},
	mcp.ToolName("datadog_list_team_links"):              {"data[].id", "data[].attributes.label", "data[].attributes.url", "data[].attributes.position"},
	mcp.ToolName("datadog_get_team_link"):                {"data.id", "data.attributes.label", "data.attributes.url", "data.attributes.position"},
	mcp.ToolName("datadog_get_team_permission_settings"): {"data[].id", "data[].attributes.action", "data[].attributes.editable", "data[].attributes.title", "data[].attributes.value"},

	// ── Synthetics ───────────────────────────────────────────────────
	mcp.ToolName("datadog_list_synthetics_tests"):      {"tests[].public_id", "tests[].name", "tests[].type", "tests[].status", "tests[].tags", "tests[].locations"},
	mcp.ToolName("datadog_get_synthetics_api_test"):    {"public_id", "name", "type", "status", "tags", "config", "locations", "message"},
	mcp.ToolName("datadog_get_synthetics_test_result"): {"result_id", "status", "check_time", "result"},

	// ── Notebooks ────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_notebooks"): {"data[].id", "data[].attributes.name", "data[].attributes.author.handle", "data[].attributes.status", "data[].attributes.created", "data[].attributes.modified"},
	mcp.ToolName("datadog_get_notebook"):   {"data.id", "data.attributes.name", "data.attributes.cells", "data.attributes.author.handle", "data.attributes.status", "data.attributes.created", "data.attributes.modified"},

	// ── Users ────────────────────────────────────────────────────────
	mcp.ToolName("datadog_list_users"): {"data[].id", "data[].attributes.name", "data[].attributes.handle", "data[].attributes.email", "data[].attributes.status", "data[].attributes.disabled"},
	mcp.ToolName("datadog_get_user"):   {"data.id", "data.attributes.name", "data.attributes.handle", "data.attributes.email", "data.attributes.status", "data.attributes.disabled"},

	// ── APM ──────────────────────────────────────────────────────────
	mcp.ToolName("datadog_search_spans"):  {"data[].attributes.service", "data[].attributes.resource_name", "data[].attributes.status", "data[].attributes.duration", "data[].attributes.start", "data[].attributes.span_id", "data[].attributes.trace_id"},
	mcp.ToolName("datadog_list_services"): {"data[].id", "data[].attributes.schema.dd-service", "data[].attributes.schema.team", "data[].attributes.schema.description"},

	// ── IP Ranges ────────────────────────────────────────────────────
	mcp.ToolName("datadog_get_ip_ranges"): {"agents", "api", "logs", "webhooks", "synthetics"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("datadog: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
