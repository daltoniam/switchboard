package clickhouse

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Queries ---
	{
		Name:        mcp.ToolName("clickhouse_execute_query"),
		Description: "Execute a SQL query (SELECT, SHOW, DESCRIBE, or DDL) against a ClickHouse analytics database and return results as JSON rows",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "SQL query string to execute", Required: true}, {Name: mcp.ParamName("database"), Description: "Optional database context to run the query in (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("clickhouse_explain_query"),
		Description: "Run EXPLAIN on a query to show the execution plan",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "SQL query to explain", Required: true}, {Name: mcp.ParamName("type"), Description: "EXPLAIN type: PLAN (default), PIPELINE, SYNTAX, AST, ESTIMATE"}},
	},

	// --- Databases ---
	{
		Name:        mcp.ToolName("clickhouse_list_databases"),
		Description: "List all databases in the ClickHouse server. Start here for schema discovery.",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_tables"),
		Description: "List all tables in a database with engine, row count, and size",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to current database)"}},
	},
	{
		Name:        mcp.ToolName("clickhouse_describe_table"),
		Description: "Describe a table's columns with names, types, default expressions, and comments",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to current database)"}, {Name: mcp.ParamName("table"), Description: "Table name", Required: true}},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_columns"),
		Description: "List detailed column metadata for a table from system.columns",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to current database)"}, {Name: mcp.ParamName("table"), Description: "Table name", Required: true}},
	},
	{
		Name:        mcp.ToolName("clickhouse_show_create_table"),
		Description: "Show the CREATE TABLE statement for a table",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to current database)"}, {Name: mcp.ParamName("table"), Description: "Table name",

		// --- System Info ---
		Required: true}},
	},

	{
		Name:        mcp.ToolName("clickhouse_server_info"),
		Description: "Get ClickHouse server version, uptime, and OS info",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_processes"),
		Description: "List currently running queries/processes",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_kill_query"),
		Description: "Kill a running query by its query ID",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query_id"), Description: "The query_id of the query to kill", Required: true}},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_settings"),
		Description: "List ClickHouse server settings. Optionally filter by name pattern",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("pattern"), Description: "Optional LIKE pattern to filter setting names (e.g. '%memory%')"}},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_merges"),
		Description: "List currently running background merges for MergeTree tables",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_replicas"),
		Description: "List replica status for replicated tables",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_disk_usage"),
		Description: "Show disk usage per database (total bytes, row counts, part counts)",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_parts"),
		Description: "List table parts with sizes for a given table (useful for debugging MergeTree)",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to current database)"}, {Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("active"), Description: "Only show active parts (default: true)"}},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_dictionaries"),
		Description: "List external dictionaries loaded in the server",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_users"),
		Description: "List all users configured in ClickHouse",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_roles"),
		Description: "List all roles configured in ClickHouse",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("clickhouse_query_log"),
		Description: "Search recent entries from system.query_log",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query_pattern"), Description: "Optional LIKE pattern to filter by query text"}, {Name: mcp.ParamName("query_type"), Description: "Filter by type: QueryStart, QueryFinish, ExceptionBeforeStart, ExceptionWhileProcessing"}, {Name: mcp.ParamName("limit"), Description: "Max rows to return (default: 50)"}},
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
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
