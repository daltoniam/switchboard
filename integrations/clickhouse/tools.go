package clickhouse

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Queries ---
	{
		Name:        "clickhouse_execute_query",
		Description: "Execute a SQL query (SELECT, SHOW, DESCRIBE, or DDL) against ClickHouse and return results as JSON rows",
		Parameters: map[string]string{
			"query":    "SQL query string to execute",
			"database": "Optional database context to run the query in (overrides configured default)",
		},
		Required: []string{"query"},
	},
	{
		Name:        "clickhouse_explain_query",
		Description: "Run EXPLAIN on a query to show the execution plan",
		Parameters: map[string]string{
			"query": "SQL query to explain",
			"type":  "EXPLAIN type: PLAN (default), PIPELINE, SYNTAX, AST, ESTIMATE",
		},
		Required: []string{"query"},
	},

	// --- Databases ---
	{
		Name:        "clickhouse_list_databases",
		Description: "List all databases in the ClickHouse server",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_list_tables",
		Description: "List all tables in a database with engine, row count, and size",
		Parameters: map[string]string{
			"database": "Database name (defaults to current database)",
		},
	},
	{
		Name:        "clickhouse_describe_table",
		Description: "Describe a table's columns with names, types, default expressions, and comments",
		Parameters: map[string]string{
			"database": "Database name (defaults to current database)",
			"table":    "Table name",
		},
		Required: []string{"table"},
	},
	{
		Name:        "clickhouse_list_columns",
		Description: "List detailed column metadata for a table from system.columns",
		Parameters: map[string]string{
			"database": "Database name (defaults to current database)",
			"table":    "Table name",
		},
		Required: []string{"table"},
	},
	{
		Name:        "clickhouse_show_create_table",
		Description: "Show the CREATE TABLE statement for a table",
		Parameters: map[string]string{
			"database": "Database name (defaults to current database)",
			"table":    "Table name",
		},
		Required: []string{"table"},
	},

	// --- System Info ---
	{
		Name:        "clickhouse_server_info",
		Description: "Get ClickHouse server version, uptime, and OS info",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_list_processes",
		Description: "List currently running queries/processes",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_kill_query",
		Description: "Kill a running query by its query ID",
		Parameters: map[string]string{
			"query_id": "The query_id of the query to kill",
		},
		Required: []string{"query_id"},
	},
	{
		Name:        "clickhouse_list_settings",
		Description: "List ClickHouse server settings. Optionally filter by name pattern",
		Parameters: map[string]string{
			"pattern": "Optional LIKE pattern to filter setting names (e.g. '%memory%')",
		},
	},
	{
		Name:        "clickhouse_list_merges",
		Description: "List currently running background merges for MergeTree tables",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_list_replicas",
		Description: "List replica status for replicated tables",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_disk_usage",
		Description: "Show disk usage per database (total bytes, row counts, part counts)",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_list_parts",
		Description: "List table parts with sizes for a given table (useful for debugging MergeTree)",
		Parameters: map[string]string{
			"database": "Database name (defaults to current database)",
			"table":    "Table name",
			"active":   "Only show active parts (default: true)",
		},
		Required: []string{"table"},
	},
	{
		Name:        "clickhouse_list_dictionaries",
		Description: "List external dictionaries loaded in the server",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_list_users",
		Description: "List all users configured in ClickHouse",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_list_roles",
		Description: "List all roles configured in ClickHouse",
		Parameters:  map[string]string{},
	},
	{
		Name:        "clickhouse_query_log",
		Description: "Search recent entries from system.query_log",
		Parameters: map[string]string{
			"query_pattern": "Optional LIKE pattern to filter by query text",
			"query_type":    "Filter by type: QueryStart, QueryFinish, ExceptionBeforeStart, ExceptionWhileProcessing",
			"limit":         "Max rows to return (default: 50)",
		},
	},
}

var dispatch = map[string]handlerFunc{
	// Queries
	"clickhouse_execute_query": executeQuery,
	"clickhouse_explain_query": explainQuery,

	// Databases
	"clickhouse_list_databases":    listDatabases,
	"clickhouse_list_tables":       listTables,
	"clickhouse_describe_table":    describeTable,
	"clickhouse_list_columns":      listColumns,
	"clickhouse_show_create_table": showCreateTable,

	// System
	"clickhouse_server_info":       serverInfo,
	"clickhouse_list_processes":    listProcesses,
	"clickhouse_kill_query":        killQuery,
	"clickhouse_list_settings":     listSettings,
	"clickhouse_list_merges":       listMerges,
	"clickhouse_list_replicas":     listReplicas,
	"clickhouse_disk_usage":        diskUsage,
	"clickhouse_list_parts":        listParts,
	"clickhouse_list_dictionaries": listDictionaries,
	"clickhouse_list_users":        listUsers,
	"clickhouse_list_roles":        listRoles,
	"clickhouse_query_log":         queryLog,
}
