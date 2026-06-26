package snowflake

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)

var dispatch = map[mcp.ToolName]handlerFunc{
	// Queries
	mcp.ToolName("snowflake_execute_query"):    executeQuery,
	mcp.ToolName("snowflake_get_query_status"): getQueryStatus,
	mcp.ToolName("snowflake_cancel_query"):     cancelQuery,

	// Schema Discovery
	mcp.ToolName("snowflake_list_databases"):    listDatabases,
	mcp.ToolName("snowflake_list_schemas"):      listSchemas,
	mcp.ToolName("snowflake_list_tables"):       listTables,
	mcp.ToolName("snowflake_list_views"):        listViews,
	mcp.ToolName("snowflake_describe_table"):    describeTable,
	mcp.ToolName("snowflake_show_create_table"): showCreateTable,

	// Warehouse & Compute
	mcp.ToolName("snowflake_list_warehouses"): listWarehouses,

	// System Info
	mcp.ToolName("snowflake_list_running_queries"): listRunningQueries,
	mcp.ToolName("snowflake_current_session"):      currentSession,

	// Users & Roles
	mcp.ToolName("snowflake_list_users"): listUsers,
	mcp.ToolName("snowflake_list_roles"): listRoles,

	// Stages & Storage
	mcp.ToolName("snowflake_list_stages"): listStages,

	// Tasks & Pipes
	mcp.ToolName("snowflake_list_tasks"): listTasks,
	mcp.ToolName("snowflake_list_pipes"): listPipes,

	// Streams
	mcp.ToolName("snowflake_list_streams"): listStreams,

	// Cortex Analyst
	mcp.ToolName("snowflake_cortex_analyst"): cortexAnalyst,
}
