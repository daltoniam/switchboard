package snowflake

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Queries ---
	{
		Name:        mcp.ToolName("snowflake_execute_query"),
		Description: "Execute a SQL query against a Snowflake data warehouse and return results as JSON rows. Supports SELECT, SHOW, DESCRIBE, DDL, and DML statements",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "SQL statement to execute", Required: true}, {Name: mcp.ParamName("database"), Description: "Database context (overrides configured default)"}, {Name: mcp.ParamName("schema"), Description: "Schema context (overrides configured default)"}, {Name: mcp.ParamName("warehouse"), Description: "Warehouse to use (overrides configured default)"}, {Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}, {Name: mcp.ParamName("timeout"), Description: "Query timeout in seconds (default: 60)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_get_query_status"),
		Description: "Check the status of an async Snowflake query and retrieve results when complete. Use the statement handle returned from snowflake_execute_query",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("statement_handle"), Description: "UUID statement handle from a previous query submission", Required: true}, {Name: mcp.ParamName("partition"), Description: "Partition number to fetch for large result sets (0-based)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_cancel_query"),
		Description: "Cancel a running Snowflake query by its statement handle",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("statement_handle"), Description: "UUID statement handle of the query to cancel", Required: true}},
	},

	// --- Schema Discovery ---
	{
		Name:        mcp.ToolName("snowflake_list_databases"),
		Description: "List all databases accessible in the Snowflake account. Start here for schema discovery.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_list_schemas"),
		Description: "List all schemas in a Snowflake database",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_list_tables"),
		Description: "List tables in a Snowflake database/schema with row counts and sizes",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_list_views"),
		Description: "List views in a Snowflake database/schema",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_describe_table"),
		Description: "Describe a table's columns with names, types, and constraints in Snowflake",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_show_create_table"),
		Description: "Show the DDL CREATE statement for a Snowflake table",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table"), Description: "Table name", Required: true}, {Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName(

		// --- Warehouse & Compute ---
		"role"), Description: "Role to use (overrides configured default)"}},
	},

	{
		Name:        mcp.ToolName("snowflake_list_warehouses"),
		Description: "List all warehouses in the Snowflake account with state, size, and cluster info",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},

	// --- System Info ---
	{
		Name:        mcp.ToolName("snowflake_list_running_queries"),
		Description: "List currently running and recently completed queries in Snowflake",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("limit"), Description: "Maximum number of queries to return (default: 50)"}, {Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_current_session"),
		Description: "Get current Snowflake session info including user, role, warehouse, and database",
		Parameters:  []mcp.Parameter{},
	},

	// --- Users & Roles ---
	{
		Name:        mcp.ToolName("snowflake_list_users"),
		Description: "List all users in the Snowflake account",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_list_roles"),
		Description: "List all roles in the Snowflake account",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},

	// --- Stages & Storage ---
	{
		Name:        mcp.ToolName("snowflake_list_stages"),
		Description: "List stages in a Snowflake database/schema for data loading",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName("role"),

		// --- Tasks & Pipes ---
		Description: "Role to use (overrides configured default)"}},
	},

	{
		Name:        mcp.ToolName("snowflake_list_tasks"),
		Description: "List tasks (scheduled SQL jobs) in a Snowflake database/schema",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName("role"), Description: "Role to use (overrides configured default)"}},
	},
	{
		Name:        mcp.ToolName("snowflake_list_pipes"),
		Description: "List Snowpipe definitions for continuous data ingestion",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName("role"),

		// --- Streams ---
		Description: "Role to use (overrides configured default)"}},
	},

	{
		Name:        mcp.ToolName("snowflake_list_streams"),
		Description: "List streams (change data capture) in a Snowflake database/schema",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database"), Description: "Database name (defaults to configured database)"}, {Name: mcp.ParamName("schema"), Description: "Schema name (defaults to configured schema)"}, {Name: mcp.ParamName("role"),

		// --- Cortex Analyst ---
		Description: "Role to use (overrides configured default)"}},
	},

	{
		Name:        mcp.ToolName("snowflake_cortex_analyst"),
		Description: "Ask a natural-language question against a Snowflake Cortex Analyst semantic layer. Returns generated SQL, an explanation, and follow-up suggestions. Use snowflake_execute_query to run the returned SQL",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("question"), Description: "Natural-language question to ask (e.g. 'What were our top 10 products by revenue last quarter?')", Required: true}, {Name: mcp.ParamName("semantic_view"), Description: "Fully qualified semantic view name (overrides configured default)"}, {Name: mcp.ParamName("semantic_model_file"), Description: "Stage path to a semantic model YAML (e.g. @MY_DB.MY_SCHEMA.MY_STAGE/model.yaml)"}, {Name: mcp.ParamName("semantic_model"), Description: "Inline semantic model YAML (alternative to semantic_model_file and semantic_view)"}},
	},
}

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
