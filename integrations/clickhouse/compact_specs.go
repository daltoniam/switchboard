package clickhouse

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Databases ────────────────────────────────────────────────────
	"clickhouse_list_databases":    {"name", "engine", "data_path"},
	"clickhouse_list_tables":       {"name", "engine", "total_rows", "total_bytes"},
	"clickhouse_describe_table":    {"name", "type", "default_kind", "default_expression", "comment"},
	"clickhouse_list_columns":      {"name", "type", "default_kind", "default_expression", "comment", "is_in_partition_key", "is_in_sorting_key"},
	"clickhouse_show_create_table": {"statement"},

	// ── System Info ──────────────────────────────────────────────────
	"clickhouse_server_info":       {"version", "uptime", "os_user", "hostname"},
	"clickhouse_list_processes":    {"query_id", "user", "query", "elapsed", "read_rows", "memory_usage"},
	"clickhouse_list_settings":     {"name", "value", "changed", "description"},
	"clickhouse_list_merges":       {"database", "table", "elapsed", "progress", "total_size_bytes_compressed"},
	"clickhouse_list_replicas":     {"database", "table", "is_leader", "total_replicas", "active_replicas", "queue_size"},
	"clickhouse_disk_usage":        {"database", "total_bytes", "total_rows", "part_count"},
	"clickhouse_list_parts":        {"name", "partition", "rows", "bytes_on_disk", "modification_time", "active"},
	"clickhouse_list_dictionaries": {"name", "type", "status", "element_count", "bytes_allocated"},
	"clickhouse_list_users":        {"name", "storage", "auth_type"},
	"clickhouse_list_roles":        {"name"},
	"clickhouse_query_log":         {"query_id", "type", "user", "query", "query_duration_ms", "read_rows", "memory_usage", "event_time"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("clickhouse: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
