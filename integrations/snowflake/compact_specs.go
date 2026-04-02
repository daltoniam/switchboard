package snowflake

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── Schema Discovery ────────────────────────────────────────────
	"snowflake_list_databases":    {"name", "owner", "created_on", "options"},
	"snowflake_list_schemas":      {"name", "database_name", "owner", "created_on"},
	"snowflake_list_tables":       {"name", "database_name", "schema_name", "rows", "bytes", "owner", "created_on"},
	"snowflake_list_views":        {"name", "database_name", "schema_name", "owner", "created_on", "text"},
	"snowflake_describe_table":    {"name", "type", "kind", "null?", "default", "primary key", "comment"},
	"snowflake_show_create_table": {"GET_DDL('TABLE', ?)"},

	// ── Warehouse & Compute ─────────────────────────────────────────
	"snowflake_list_warehouses": {"name", "state", "type", "size", "running", "queued", "owner"},

	// ── System Info ─────────────────────────────────────────────────
	"snowflake_list_running_queries": {"query_id", "query_text", "user_name", "warehouse_name", "execution_status", "total_elapsed_time"},
	"snowflake_current_session":      {"current_user", "current_role", "current_warehouse", "current_database", "current_schema"},

	// ── Users & Roles ───────────────────────────────────────────────
	"snowflake_list_users": {"name", "login_name", "display_name", "default_role", "owner", "disabled"},
	"snowflake_list_roles": {"name", "owner", "assigned_to_users", "granted_to_roles", "granted_roles"},

	// ── Stages, Tasks, Pipes, Streams ───────────────────────────────
	"snowflake_list_stages":  {"name", "database_name", "schema_name", "type", "owner", "url"},
	"snowflake_list_tasks":   {"name", "database_name", "schema_name", "schedule", "state", "definition", "owner"},
	"snowflake_list_pipes":   {"name", "database_name", "schema_name", "definition", "owner"},
	"snowflake_list_streams": {"name", "database_name", "schema_name", "table_name", "type", "stale", "mode"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("snowflake: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
