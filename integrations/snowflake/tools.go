package snowflake

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Queries ---
	{
		Name:        "snowflake_execute_query",
		Description: "Execute a SQL query against a Snowflake data warehouse and return results as JSON rows. Supports SELECT, SHOW, DESCRIBE, DDL, and DML statements",
		Parameters: map[string]string{
			"query":     "SQL statement to execute",
			"database":  "Database context (overrides configured default)",
			"schema":    "Schema context (overrides configured default)",
			"warehouse": "Warehouse to use (overrides configured default)",
			"role":      "Role to use (overrides configured default)",
			"timeout":   "Query timeout in seconds (default: 60)",
		},
		Required: []string{"query"},
	},
	{
		Name:        "snowflake_get_query_status",
		Description: "Check the status of an async Snowflake query and retrieve results when complete. Use the statement handle returned from snowflake_execute_query",
		Parameters: map[string]string{
			"statement_handle": "UUID statement handle from a previous query submission",
			"partition":        "Partition number to fetch for large result sets (0-based)",
		},
		Required: []string{"statement_handle"},
	},
	{
		Name:        "snowflake_cancel_query",
		Description: "Cancel a running Snowflake query by its statement handle",
		Parameters: map[string]string{
			"statement_handle": "UUID statement handle of the query to cancel",
		},
		Required: []string{"statement_handle"},
	},

	// --- Schema Discovery ---
	{
		Name:        "snowflake_list_databases",
		Description: "List all databases accessible in the Snowflake account",
		Parameters: map[string]string{
			"role": "Role to use (overrides configured default)",
		},
	},
	{
		Name:        "snowflake_list_schemas",
		Description: "List all schemas in a Snowflake database",
		Parameters: map[string]string{
			"database": "Database name (defaults to configured database)",
			"role":     "Role to use (overrides configured default)",
		},
	},
	{
		Name:        "snowflake_list_tables",
		Description: "List tables in a Snowflake database/schema with row counts and sizes",
		Parameters: map[string]string{
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
	},
	{
		Name:        "snowflake_list_views",
		Description: "List views in a Snowflake database/schema",
		Parameters: map[string]string{
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
	},
	{
		Name:        "snowflake_describe_table",
		Description: "Describe a table's columns with names, types, and constraints in Snowflake",
		Parameters: map[string]string{
			"table":    "Table name",
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
		Required: []string{"table"},
	},
	{
		Name:        "snowflake_show_create_table",
		Description: "Show the DDL CREATE statement for a Snowflake table",
		Parameters: map[string]string{
			"table":    "Table name",
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
		Required: []string{"table"},
	},

	// --- Warehouse & Compute ---
	{
		Name:        "snowflake_list_warehouses",
		Description: "List all warehouses in the Snowflake account with state, size, and cluster info",
		Parameters: map[string]string{
			"role": "Role to use (overrides configured default)",
		},
	},

	// --- System Info ---
	{
		Name:        "snowflake_list_running_queries",
		Description: "List currently running and recently completed queries in Snowflake",
		Parameters: map[string]string{
			"limit": "Maximum number of queries to return (default: 50)",
			"role":  "Role to use (overrides configured default)",
		},
	},
	{
		Name:        "snowflake_current_session",
		Description: "Get current Snowflake session info including user, role, warehouse, and database",
		Parameters:  map[string]string{},
	},

	// --- Users & Roles ---
	{
		Name:        "snowflake_list_users",
		Description: "List all users in the Snowflake account",
		Parameters: map[string]string{
			"role": "Role to use (overrides configured default)",
		},
	},
	{
		Name:        "snowflake_list_roles",
		Description: "List all roles in the Snowflake account",
		Parameters: map[string]string{
			"role": "Role to use (overrides configured default)",
		},
	},

	// --- Stages & Storage ---
	{
		Name:        "snowflake_list_stages",
		Description: "List stages in a Snowflake database/schema for data loading",
		Parameters: map[string]string{
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
	},

	// --- Tasks & Pipes ---
	{
		Name:        "snowflake_list_tasks",
		Description: "List tasks (scheduled SQL jobs) in a Snowflake database/schema",
		Parameters: map[string]string{
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
	},
	{
		Name:        "snowflake_list_pipes",
		Description: "List Snowpipe definitions for continuous data ingestion",
		Parameters: map[string]string{
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
	},

	// --- Streams ---
	{
		Name:        "snowflake_list_streams",
		Description: "List streams (change data capture) in a Snowflake database/schema",
		Parameters: map[string]string{
			"database": "Database name (defaults to configured database)",
			"schema":   "Schema name (defaults to configured schema)",
			"role":     "Role to use (overrides configured default)",
		},
	},
}

var dispatch = map[string]handlerFunc{
	// Queries
	"snowflake_execute_query":    executeQuery,
	"snowflake_get_query_status": getQueryStatus,
	"snowflake_cancel_query":     cancelQuery,

	// Schema Discovery
	"snowflake_list_databases":    listDatabases,
	"snowflake_list_schemas":      listSchemas,
	"snowflake_list_tables":       listTables,
	"snowflake_list_views":        listViews,
	"snowflake_describe_table":    describeTable,
	"snowflake_show_create_table": showCreateTable,

	// Warehouse & Compute
	"snowflake_list_warehouses": listWarehouses,

	// System Info
	"snowflake_list_running_queries": listRunningQueries,
	"snowflake_current_session":      currentSession,

	// Users & Roles
	"snowflake_list_users": listUsers,
	"snowflake_list_roles": listRoles,

	// Stages & Storage
	"snowflake_list_stages": listStages,

	// Tasks & Pipes
	"snowflake_list_tasks": listTasks,
	"snowflake_list_pipes": listPipes,

	// Streams
	"snowflake_list_streams": listStreams,
}
