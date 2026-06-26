package clickhouse

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("clickhouse_list_connections"): listConnections,

	// Queries
	mcp.ToolName("clickhouse_execute_query"): executeQuery,
	mcp.ToolName("clickhouse_explain_query"): explainQuery,

	// Databases
	mcp.ToolName("clickhouse_list_databases"):    listDatabases,
	mcp.ToolName("clickhouse_list_tables"):       listTables,
	mcp.ToolName("clickhouse_describe_table"):    describeTable,
	mcp.ToolName("clickhouse_list_columns"):      listColumns,
	mcp.ToolName("clickhouse_show_create_table"): showCreateTable,

	// System
	mcp.ToolName("clickhouse_server_info"):       serverInfo,
	mcp.ToolName("clickhouse_list_processes"):    listProcesses,
	mcp.ToolName("clickhouse_kill_query"):        killQuery,
	mcp.ToolName("clickhouse_list_settings"):     listSettings,
	mcp.ToolName("clickhouse_list_merges"):       listMerges,
	mcp.ToolName("clickhouse_list_replicas"):     listReplicas,
	mcp.ToolName("clickhouse_disk_usage"):        diskUsage,
	mcp.ToolName("clickhouse_list_parts"):        listParts,
	mcp.ToolName("clickhouse_list_dictionaries"): listDictionaries,
	mcp.ToolName("clickhouse_list_users"):        listUsers,
	mcp.ToolName("clickhouse_list_roles"):        listRoles,
	mcp.ToolName("clickhouse_query_log"):         queryLog,
}
