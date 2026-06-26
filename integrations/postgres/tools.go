package postgres

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)

var dispatch = map[mcp.ToolName]handlerFunc{
	// Connection Management
	mcp.ToolName("postgres_list_databases"): listDatabases,

	// Schema Discovery
	mcp.ToolName("postgres_list_schemas"):      listSchemas,
	mcp.ToolName("postgres_list_tables"):       listTables,
	mcp.ToolName("postgres_describe_table"):    describeTable,
	mcp.ToolName("postgres_list_columns"):      listColumns,
	mcp.ToolName("postgres_list_indexes"):      listIndexes,
	mcp.ToolName("postgres_list_constraints"):  listConstraints,
	mcp.ToolName("postgres_list_foreign_keys"): listForeignKeys,
	mcp.ToolName("postgres_list_views"):        listViews,
	mcp.ToolName("postgres_list_functions"):    listFunctions,
	mcp.ToolName("postgres_list_triggers"):     listTriggers,
	mcp.ToolName("postgres_list_enums"):        listEnums,

	// Query Execution
	mcp.ToolName("postgres_query"):   queryTool,
	mcp.ToolName("postgres_execute"): executeTool,
	mcp.ToolName("postgres_explain"): explainTool,

	// Table Data
	mcp.ToolName("postgres_select"): selectTool,

	// Database Info
	mcp.ToolName("postgres_database_info"): databaseInfo,
	mcp.ToolName("postgres_database_size"): databaseSize,
	mcp.ToolName("postgres_table_stats"):   tableStats,

	// Roles & Permissions
	mcp.ToolName("postgres_list_roles"):  listRoles,
	mcp.ToolName("postgres_list_grants"): listGrants,

	// Extensions & Connections
	mcp.ToolName("postgres_list_extensions"):         listExtensions,
	mcp.ToolName("postgres_list_active_connections"): listActiveConnections,
	mcp.ToolName("postgres_list_locks"):              listLocks,
	mcp.ToolName("postgres_running_queries"):         runningQueries,
}
