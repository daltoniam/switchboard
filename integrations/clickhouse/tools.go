package clickhouse

import mcp "github.com/daltoniam/switchboard"

const connectionParamDesc = "Connection alias (omit to use default). Use clickhouse_list_connections to see configured clusters."

var tools = []mcp.ToolDefinition{
	{
		Name:        mcp.ToolName("clickhouse_list_connections"),
		Description: "List all configured ClickHouse cluster connections with their alias, host, database, TLS, and default status. Start here when choosing which cluster to query.",
		Parameters:  map[string]string{},
	},

	// --- Queries ---
	{
		Name:        mcp.ToolName("clickhouse_execute_query"),
		Description: "Execute a SQL query (SELECT, SHOW, DESCRIBE, or DDL) against a ClickHouse analytics database and return results as JSON rows",
		Parameters: map[string]string{
			"query":      "SQL query string to execute",
			"database":   "Optional ClickHouse database context to run the query in (overrides configured default)",
			"connection": connectionParamDesc,
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("clickhouse_explain_query"),
		Description: "Run EXPLAIN on a query to show the execution plan",
		Parameters: map[string]string{
			"query":      "SQL query to explain",
			"type":       "EXPLAIN type: PLAN (default), PIPELINE, SYNTAX, AST, ESTIMATE",
			"connection": connectionParamDesc,
		},
		Required: []string{"query"},
	},

	// --- Databases ---
	{
		Name:        mcp.ToolName("clickhouse_list_databases"),
		Description: "List all databases in a ClickHouse cluster. Start here for schema discovery after choosing a connection.",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_tables"),
		Description: "List all tables in a ClickHouse database with engine, row count, and size",
		Parameters: map[string]string{
			"database":   "ClickHouse database name (defaults to current database)",
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_describe_table"),
		Description: "Describe a table's columns with names, types, default expressions, and comments",
		Parameters: map[string]string{
			"database":   "ClickHouse database name (defaults to current database)",
			"table":      "Table name",
			"connection": connectionParamDesc,
		},
		Required: []string{"table"},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_columns"),
		Description: "List detailed column metadata for a table from system.columns",
		Parameters: map[string]string{
			"database":   "ClickHouse database name (defaults to current database)",
			"table":      "Table name",
			"connection": connectionParamDesc,
		},
		Required: []string{"table"},
	},
	{
		Name:        mcp.ToolName("clickhouse_show_create_table"),
		Description: "Show the CREATE TABLE statement for a table",
		Parameters: map[string]string{
			"database":   "ClickHouse database name (defaults to current database)",
			"table":      "Table name",
			"connection": connectionParamDesc,
		},
		Required: []string{"table"},
	},

	// --- System Info ---
	{
		Name:        mcp.ToolName("clickhouse_server_info"),
		Description: "Get ClickHouse server version, uptime, and OS info",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_processes"),
		Description: "List currently running queries/processes",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_kill_query"),
		Description: "Kill a running query by its query ID",
		Parameters: map[string]string{
			"query_id":   "The query_id of the query to kill",
			"connection": connectionParamDesc,
		},
		Required: []string{"query_id"},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_settings"),
		Description: "List ClickHouse server settings. Optionally filter by name pattern",
		Parameters: map[string]string{
			"pattern":    "Optional LIKE pattern to filter setting names (e.g. '%memory%')",
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_merges"),
		Description: "List currently running background merges for MergeTree tables",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_replicas"),
		Description: "List replica status for replicated tables",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_disk_usage"),
		Description: "Show disk usage per ClickHouse database (total bytes, row counts, part counts)",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_parts"),
		Description: "List table parts with sizes for a given table (useful for debugging MergeTree)",
		Parameters: map[string]string{
			"database":   "ClickHouse database name (defaults to current database)",
			"table":      "Table name",
			"active":     "Only show active parts (default: true)",
			"connection": connectionParamDesc,
		},
		Required: []string{"table"},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_dictionaries"),
		Description: "List external dictionaries loaded in the server",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_users"),
		Description: "List all users configured in ClickHouse",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_list_roles"),
		Description: "List all roles configured in ClickHouse",
		Parameters: map[string]string{
			"connection": connectionParamDesc,
		},
	},
	{
		Name:        mcp.ToolName("clickhouse_query_log"),
		Description: "Search recent entries from system.query_log",
		Parameters: map[string]string{
			"query_pattern": "Optional LIKE pattern to filter by query text",
			"query_type":    "Filter by type: QueryStart, QueryFinish, ExceptionBeforeStart, ExceptionWhileProcessing",
			"limit":         "Max rows to return (default: 50)",
			"connection":    connectionParamDesc,
		},
	},
}

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
