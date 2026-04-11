package clickhouse

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Databases ────────────────────────────────────────────────────
	mcp.ToolName("clickhouse_list_databases"):    {"name", "engine", "data_path"},
	mcp.ToolName("clickhouse_list_tables"):       {"name", "engine", "total_rows", "total_bytes"},
	mcp.ToolName("clickhouse_describe_table"):    {"name", "type", "default_kind", "default_expression", "comment"},
	mcp.ToolName("clickhouse_list_columns"):      {"name", "type", "default_kind", "default_expression", "comment", "is_in_partition_key", "is_in_sorting_key"},
	mcp.ToolName("clickhouse_show_create_table"): {"statement"},

	// ── System Info ──────────────────────────────────────────────────
	mcp.ToolName("clickhouse_server_info"):       {"version", "uptime", "os_user", "hostname"},
	mcp.ToolName("clickhouse_list_processes"):    {"query_id", "user", "query", "elapsed", "read_rows", "memory_usage"},
	mcp.ToolName("clickhouse_list_settings"):     {"name", "value", "changed", "description"},
	mcp.ToolName("clickhouse_list_merges"):       {"database", "table", "elapsed", "progress", "total_size_bytes_compressed"},
	mcp.ToolName("clickhouse_list_replicas"):     {"database", "table", "is_leader", "total_replicas", "active_replicas", "queue_size"},
	mcp.ToolName("clickhouse_disk_usage"):        {"database", "total_bytes", "total_rows", "part_count"},
	mcp.ToolName("clickhouse_list_parts"):        {"name", "partition", "rows", "bytes_on_disk", "modification_time", "active"},
	mcp.ToolName("clickhouse_list_dictionaries"): {"name", "type", "status", "element_count", "bytes_allocated"},
	mcp.ToolName("clickhouse_list_users"):        {"name", "storage", "auth_type"},
	mcp.ToolName("clickhouse_list_roles"):        {"name"},
	mcp.ToolName("clickhouse_query_log"):         {"query_id", "type", "user", "query", "query_duration_ms", "read_rows", "memory_usage", "event_time"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("clickhouse: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
