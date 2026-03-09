package postgres

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Schema Discovery ─────────────────────────────────────────────
	"postgres_list_schemas":      {"schema_name"},
	"postgres_list_tables":       {"table_name", "schema_name", "row_count", "total_size"},
	"postgres_describe_table":    {"column_name", "data_type", "is_nullable", "column_default", "constraints"},
	"postgres_list_columns":      {"column_name", "data_type", "ordinal_position", "is_nullable"},
	"postgres_list_indexes":      {"indexname", "indexdef", "tablespace", "size"},
	"postgres_list_constraints":  {"constraint_name", "constraint_type", "column_name", "foreign_table"},
	"postgres_list_foreign_keys": {"constraint_name", "column_name", "foreign_table_name", "foreign_column_name"},
	"postgres_list_views":        {"viewname", "definition"},
	"postgres_list_functions":    {"function_name", "result_type", "argument_types", "type"},
	"postgres_list_triggers":     {"trigger_name", "event_manipulation", "event_object_table", "action_statement"},
	"postgres_list_enums":        {"enum_name", "enum_values"},

	// ── Database Info ────────────────────────────────────────────────
	"postgres_database_info": {"version", "current_database", "current_user", "server_encoding"},
	"postgres_database_size": {"database_size", "tables"},
	"postgres_table_stats":   {"table_name", "row_count", "dead_tuples", "last_vacuum", "last_analyze"},

	// ── Roles & Permissions ──────────────────────────────────────────
	"postgres_list_roles":  {"rolname", "rolsuper", "rolcreatedb", "rolcanlogin"},
	"postgres_list_grants": {"grantee", "privilege_type", "table_name"},

	// ── Extensions & Connections ─────────────────────────────────────
	"postgres_list_extensions":         {"name", "default_version", "installed_version", "comment"},
	"postgres_list_active_connections": {"pid", "usename", "datname", "state", "query", "query_start", "client_addr"},
	"postgres_list_locks":              {"pid", "locktype", "relation", "mode", "granted", "query"},
	"postgres_running_queries":         {"pid", "usename", "state", "query", "duration"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("postgres: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
