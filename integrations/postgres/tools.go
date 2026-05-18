package postgres

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Connection Management ---
	{
		Name:        "postgres_list_databases",
		Description: "List all configured database connections with their alias, host, database name, and read-only status",
		Parameters:  []mcp.Parameter{},
	},

	// --- Schema Discovery ---
	{
		Name:        mcp.ToolName("postgres_list_schemas"),
		Description: "List all schemas in the database. Start here for schema discovery.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_tables"),
		Description: "List all tables in a schema with row counts and size estimates",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_describe_table"),
		Description: "Get detailed column info for a table including types, nullability, defaults, and constraints",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_columns"),
		Description: "List all columns for a table with data types and ordinal positions",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_indexes"),
		Description: "List all indexes on a table with definitions and size",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_constraints"),
		Description: "List all constraints (primary key, foreign key, unique, check) on a table",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_foreign_keys"),
		Description: "List all foreign key relationships for a table (both referencing and referenced)",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_views"),
		Description: "List all views in a schema with their definitions",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_functions"),
		Description: "List user-defined functions in a schema",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_triggers"),
		Description: "List all triggers on a table or in a schema",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name (optional, lists all triggers in schema if omitted)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_enums"),
		Description: "List all enum types in the database with their values",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},

	{
		Name:        mcp.ToolName("postgres_query"),
		Description: "Execute a read-only SQL query and return results as JSON. Use for database exploration and performance investigation. Automatically wrapped in a read-only transaction.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("sql"), Description: "SQL query to execute (SELECT, SHOW, EXPLAIN, etc.)", Required: true}, {Name: mcp.ParamName("limit"), Description: "Max rows to return (default: 100, max: 1000)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_execute"),
		Description: "Execute a data-modifying SQL statement (INSERT, UPDATE, DELETE, CREATE, ALTER, DROP). Returns rows affected. **CAUTION: executes arbitrary SQL including DDL/DML. Disabled by default -- set read_only=false in credentials to enable.** DROP DATABASE and TRUNCATE are always denied.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("sql"), Description: "SQL statement to execute", Required: true}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_explain"),
		Description: "Run EXPLAIN ANALYZE on a SQL query to show the execution plan with actual timing. Use to diagnose slow queries and optimize database performance.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("sql"), Description: "SQL query to explain", Required: true}, {Name: mcp.ParamName("analyze"), Description: "Run EXPLAIN ANALYZE with actual execution (default: false)"}, {Name: mcp.ParamName("format"), Description: "Output format: text, json, yaml, xml (default: text)"},

		// --- Table Data ---
		{Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},

	{
		Name:        mcp.ToolName("postgres_select"),
		Description: "Select rows from a table with optional filtering, ordering, and pagination. The columns, where, and order_by parameters accept SQL expressions (semicolons and comments are rejected).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("columns"), Description: "Comma-separated column names or SQL expressions (default: *)"}, {Name: mcp.ParamName("where"), Description: "WHERE clause without the WHERE keyword (SQL expression)"}, {Name: mcp.ParamName("order_by"), Description: "ORDER BY clause without the ORDER BY keyword (SQL expression)"}, {Name: mcp.ParamName("limit"),

		// --- Database Info ---
		Description: "Max rows to return (default: 100)"}, {Name: mcp.ParamName("offset"), Description: "Number of rows to skip"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},

	{
		Name:        mcp.ToolName("postgres_database_info"),
		Description: "Get database-level info: version, current database, current user, server settings",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_database_size"),
		Description: "Get the size of the current database and its largest tables",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("limit"), Description: "Number of largest tables to return (default: 20)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_table_stats"),
		Description: "Get detailed statistics for a table including row count, dead tuples, last vacuum/analyze times",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},

	// --- Roles & Permissions ---
	{
		Name:        mcp.ToolName("postgres_list_roles"),
		Description: "List all database roles with their attributes (superuser, createdb, login, etc.)",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_grants"),
		Description: "List privileges granted on a table or schema",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name (optional, shows schema-level grants if omitted)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (default: public)"},

		// --- Extensions & Connections ---
		{Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},

	{
		Name:        mcp.ToolName("postgres_list_extensions"),
		Description: "List all installed extensions with versions",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_active_connections"),
		Description: "List active database connections with query state, duration, and client info",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("state"), Description: "Filter by state: active, idle, idle in transaction (optional)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_list_locks"),
		Description: "List current lock activity showing blocked and blocking queries",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
	{
		Name:        mcp.ToolName("postgres_running_queries"),
		Description: "List currently running queries with duration and state. Use to find slow or long-running queries that may be blocking database operations.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("min_duration"), Description: "Minimum duration in seconds to filter by (optional)"}, {Name: mcp.ParamName("database"), Description: "Connection alias (omit to use default). Use postgres_list_databases to see available connections."}},
	},
}

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
