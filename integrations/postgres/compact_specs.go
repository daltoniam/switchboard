package postgres

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Connection Management ────────────────────────────────────────
	mcp.ToolName("postgres_list_databases"): {"alias", "host", "database", "read_only", "is_default"},

	// ── Schema Discovery ─────────────────────────────────────────────
	mcp.ToolName("postgres_list_schemas"):      {"schema_name"},
	mcp.ToolName("postgres_list_tables"):       {"table_name", "schema_name", "row_count", "total_size"},
	mcp.ToolName("postgres_describe_table"):    {"column_name", "data_type", "is_nullable", "column_default", "constraints"},
	mcp.ToolName("postgres_list_columns"):      {"column_name", "data_type", "ordinal_position", "is_nullable"},
	mcp.ToolName("postgres_list_indexes"):      {"indexname", "indexdef", "tablespace", "size"},
	mcp.ToolName("postgres_list_constraints"):  {"constraint_name", "constraint_type", "column_name", "foreign_table"},
	mcp.ToolName("postgres_list_foreign_keys"): {"constraint_name", "column_name", "foreign_table_name", "foreign_column_name"},
	mcp.ToolName("postgres_list_views"):        {"viewname", "definition"},
	mcp.ToolName("postgres_list_functions"):    {"function_name", "result_type", "argument_types", "type"},
	mcp.ToolName("postgres_list_triggers"):     {"trigger_name", "event_manipulation", "event_object_table", "action_statement"},
	mcp.ToolName("postgres_list_enums"):        {"enum_name", "enum_values"},

	// ── Database Info ────────────────────────────────────────────────
	mcp.ToolName("postgres_database_info"): {"version", "current_database", "current_user", "server_encoding"},
	mcp.ToolName("postgres_database_size"): {"database_size", "tables"},
	mcp.ToolName("postgres_table_stats"):   {"table_name", "row_count", "dead_tuples", "last_vacuum", "last_analyze"},

	// ── Roles & Permissions ──────────────────────────────────────────
	mcp.ToolName("postgres_list_roles"):  {"rolname", "rolsuper", "rolcreatedb", "rolcanlogin"},
	mcp.ToolName("postgres_list_grants"): {"grantee", "privilege_type", "table_name"},

	// ── Extensions & Connections ─────────────────────────────────────
	mcp.ToolName("postgres_list_extensions"):         {"name", "default_version", "installed_version", "comment"},
	mcp.ToolName("postgres_list_active_connections"): {"pid", "usename", "datname", "state", "query", "query_start", "client_addr"},
	mcp.ToolName("postgres_list_locks"):              {"pid", "locktype", "relation", "mode", "granted", "query"},
	mcp.ToolName("postgres_running_queries"):         {"pid", "usename", "state", "query", "duration"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("postgres: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
