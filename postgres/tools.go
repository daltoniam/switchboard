package postgres

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Schema Discovery ---
	{
		Name:        "postgres_list_schemas",
		Description: "List all schemas in the database",
		Parameters:  map[string]string{},
	},
	{
		Name:        "postgres_list_tables",
		Description: "List all tables in a schema with row counts and size estimates",
		Parameters: map[string]string{
			"schema": "Schema name (default: public)",
		},
	},
	{
		Name:        "postgres_describe_table",
		Description: "Get detailed column info for a table including types, nullability, defaults, and constraints",
		Parameters: map[string]string{
			"table":  "Table name",
			"schema": "Schema name (default: public)",
		},
		Required: []string{"table"},
	},
	{
		Name:        "postgres_list_columns",
		Description: "List all columns for a table with data types and ordinal positions",
		Parameters: map[string]string{
			"table":  "Table name",
			"schema": "Schema name (default: public)",
		},
		Required: []string{"table"},
	},
	{
		Name:        "postgres_list_indexes",
		Description: "List all indexes on a table with definitions and size",
		Parameters: map[string]string{
			"table":  "Table name",
			"schema": "Schema name (default: public)",
		},
		Required: []string{"table"},
	},
	{
		Name:        "postgres_list_constraints",
		Description: "List all constraints (primary key, foreign key, unique, check) on a table",
		Parameters: map[string]string{
			"table":  "Table name",
			"schema": "Schema name (default: public)",
		},
		Required: []string{"table"},
	},
	{
		Name:        "postgres_list_foreign_keys",
		Description: "List all foreign key relationships for a table (both referencing and referenced)",
		Parameters: map[string]string{
			"table":  "Table name",
			"schema": "Schema name (default: public)",
		},
		Required: []string{"table"},
	},
	{
		Name:        "postgres_list_views",
		Description: "List all views in a schema with their definitions",
		Parameters: map[string]string{
			"schema": "Schema name (default: public)",
		},
	},
	{
		Name:        "postgres_list_functions",
		Description: "List user-defined functions in a schema",
		Parameters: map[string]string{
			"schema": "Schema name (default: public)",
		},
	},
	{
		Name:        "postgres_list_triggers",
		Description: "List all triggers on a table or in a schema",
		Parameters: map[string]string{
			"table":  "Table name (optional, lists all triggers in schema if omitted)",
			"schema": "Schema name (default: public)",
		},
	},
	{
		Name:        "postgres_list_enums",
		Description: "List all enum types in the database with their values",
		Parameters: map[string]string{
			"schema": "Schema name (default: public)",
		},
	},

	// --- Query Execution ---
	{
		Name:        "postgres_query",
		Description: "Execute a read-only SQL query and return results as JSON. Automatically wrapped in a read-only transaction.",
		Parameters: map[string]string{
			"sql":   "SQL query to execute (SELECT, SHOW, EXPLAIN, etc.)",
			"limit": "Max rows to return (default: 100, max: 1000)",
		},
		Required: []string{"sql"},
	},
	{
		Name:        "postgres_execute",
		Description: "Execute a data-modifying SQL statement (INSERT, UPDATE, DELETE, CREATE, ALTER, DROP). Returns rows affected.",
		Parameters: map[string]string{
			"sql": "SQL statement to execute",
		},
		Required: []string{"sql"},
	},
	{
		Name:        "postgres_explain",
		Description: "Run EXPLAIN ANALYZE on a query to show the execution plan with actual timing",
		Parameters: map[string]string{
			"sql":     "SQL query to explain",
			"analyze": "Run EXPLAIN ANALYZE with actual execution (default: false)",
			"format":  "Output format: text, json, yaml, xml (default: text)",
		},
		Required: []string{"sql"},
	},

	// --- Table Data ---
	{
		Name:        "postgres_select",
		Description: "Select rows from a table with optional filtering, ordering, and pagination",
		Parameters: map[string]string{
			"table":    "Table name",
			"schema":   "Schema name (default: public)",
			"columns":  "Comma-separated column names (default: *)",
			"where":    "WHERE clause (without the WHERE keyword)",
			"order_by": "ORDER BY clause (without the ORDER BY keyword)",
			"limit":    "Max rows to return (default: 100)",
			"offset":   "Number of rows to skip",
		},
		Required: []string{"table"},
	},

	// --- Database Info ---
	{
		Name:        "postgres_database_info",
		Description: "Get database-level info: version, current database, current user, server settings",
		Parameters:  map[string]string{},
	},
	{
		Name:        "postgres_database_size",
		Description: "Get the size of the current database and its largest tables",
		Parameters: map[string]string{
			"limit": "Number of largest tables to return (default: 20)",
		},
	},
	{
		Name:        "postgres_table_stats",
		Description: "Get detailed statistics for a table including row count, dead tuples, last vacuum/analyze times",
		Parameters: map[string]string{
			"table":  "Table name",
			"schema": "Schema name (default: public)",
		},
		Required: []string{"table"},
	},

	// --- Roles & Permissions ---
	{
		Name:        "postgres_list_roles",
		Description: "List all database roles with their attributes (superuser, createdb, login, etc.)",
		Parameters:  map[string]string{},
	},
	{
		Name:        "postgres_list_grants",
		Description: "List privileges granted on a table or schema",
		Parameters: map[string]string{
			"table":  "Table name (optional, shows schema-level grants if omitted)",
			"schema": "Schema name (default: public)",
		},
	},

	// --- Extensions & Connections ---
	{
		Name:        "postgres_list_extensions",
		Description: "List all installed extensions with versions",
		Parameters:  map[string]string{},
	},
	{
		Name:        "postgres_list_active_connections",
		Description: "List active database connections with query state, duration, and client info",
		Parameters: map[string]string{
			"state": "Filter by state: active, idle, idle in transaction (optional)",
		},
	},
	{
		Name:        "postgres_list_locks",
		Description: "List current lock activity showing blocked and blocking queries",
		Parameters:  map[string]string{},
	},
	{
		Name:        "postgres_running_queries",
		Description: "List currently running queries with duration and state",
		Parameters: map[string]string{
			"min_duration": "Minimum duration in seconds to filter by (optional)",
		},
	},
}

var dispatch = map[string]handlerFunc{
	// Schema Discovery
	"postgres_list_schemas":      listSchemas,
	"postgres_list_tables":       listTables,
	"postgres_describe_table":    describeTable,
	"postgres_list_columns":      listColumns,
	"postgres_list_indexes":      listIndexes,
	"postgres_list_constraints":  listConstraints,
	"postgres_list_foreign_keys": listForeignKeys,
	"postgres_list_views":        listViews,
	"postgres_list_functions":    listFunctions,
	"postgres_list_triggers":     listTriggers,
	"postgres_list_enums":        listEnums,

	// Query Execution
	"postgres_query":   queryTool,
	"postgres_execute": executeTool,
	"postgres_explain": explainTool,

	// Table Data
	"postgres_select": selectTool,

	// Database Info
	"postgres_database_info":  databaseInfo,
	"postgres_database_size":  databaseSize,
	"postgres_table_stats":    tableStats,

	// Roles & Permissions
	"postgres_list_roles":  listRoles,
	"postgres_list_grants": listGrants,

	// Extensions & Connections
	"postgres_list_extensions":          listExtensions,
	"postgres_list_active_connections":  listActiveConnections,
	"postgres_list_locks":               listLocks,
	"postgres_running_queries":          runningQueries,
}
