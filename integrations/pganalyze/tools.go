package pganalyze

import mcp "github.com/daltoniam/switchboard"

// staticTools defines the known pganalyze MCP tool definitions.
// These are used for search indexing when the proxy has not yet connected.
// When the proxy connects successfully, tools are dynamically refreshed from the MCP server.
var staticTools = []mcp.ToolDefinition{
	// --- Servers ---
	{
		Name:        mcp.ToolName("pganalyze_list_servers"),
		Description: "List monitored PostgreSQL servers in pganalyze. Start here to discover server IDs and database IDs needed by other tools.",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_server_details"),
		Description: "Get details for a specific monitored PostgreSQL server including configuration and snapshot info.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("server_id"), Description: "The server ID (from pganalyze_list_servers)", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_postgres_settings"),
		Description: "Get PostgreSQL configuration settings (e.g. shared_buffers, work_mem) for a server.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("server_id"), Description: "The server ID (from pganalyze_list_servers)", Required: true}},
	},

	// --- Databases ---
	{
		Name:        mcp.ToolName("pganalyze_get_databases"),
		Description: "List databases with size stats and issue counts for a server.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("server_id"), Description: "The server ID (from pganalyze_list_servers)", Required: true}},
	},

	// --- Queries ---
	{
		Name:        mcp.ToolName("pganalyze_get_query_stats"),
		Description: "Get top queries by runtime percentage. Shows expensive and slow query bottlenecks sorted by impact.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID (from pganalyze_list_servers or pganalyze_get_databases)", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of queries to return (default: 10)"}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_details"),
		Description: "Get the full normalized query text for a specific query.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("query_id"), Description: "Query ID (from pganalyze_get_query_stats)", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_samples"),
		Description: "Get sample executions for a query with runtime and parameters.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("query_id"), Description: "Query ID (from pganalyze_get_query_stats)",

		// --- Tables ---
		Required: true}},
	},

	{
		Name:        mcp.ToolName("pganalyze_get_tables"),
		Description: "List tables with filtering and pagination for a database.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("schema_name"), Description: "Filter by schema name (optional)"}, {Name: mcp.ParamName("limit"), Description: "Number of tables to return (optional)"}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_table"),
		Description: "Get detailed information about a single table: schema details, columns with per-column stats, indexes, and constraints.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("schema_name"), Description: "Schema name", Required: true}, {Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_table_stats"),
		Description: "Get time-series table statistics (row counts, dead tuples, sequential scans, etc.).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("schema_name"), Description: "Schema name", Required: true}, {Name: mcp.ParamName("table_name"), Description: "Table name", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_index_selection"),
		Description: "Get Index Advisor results for an existing run. Shows recommended indexes.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_run_index_selection"),
		Description: "Run the Index Advisor for a table to get index recommendations.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("schema_name"), Description: "Schema name", Required: true}, {Name: mcp.ParamName("table_name"),

		// --- EXPLAIN Plans ---
		Description: "Table name", Required: true}},
	},

	{
		Name:        mcp.ToolName("pganalyze_get_query_explains"),
		Description: "List EXPLAIN plans for a query (last 7 days). Use to find query plan changes and regressions.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("query_id"), Description: "Query ID (from pganalyze_get_query_stats)", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_explain"),
		Description: "Get a specific EXPLAIN plan with full output including node details and costs.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("explain_id"), Description: "EXPLAIN plan ID (from pganalyze_get_query_explains)", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_explain_from_trace"),
		Description: "Resolve an OpenTelemetry trace span to an EXPLAIN plan. Requires OpenTelemetry integration.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("trace_id"), Description: "OpenTelemetry trace ID", Required: true}, {Name: mcp.ParamName("span_id"), Description: "OpenTelemetry span ID",

		// --- Backends ---
		Required: true}},
	},

	{
		Name:        mcp.ToolName("pganalyze_get_backend_counts"),
		Description: "Get time-series connection counts by state (active, idle, waiting).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("server_id"), Description: "The server ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_backends"),
		Description: "Get a point-in-time snapshot of active connections and their states.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("server_id"), Description: "The server ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_backend_details"),
		Description: "Get details for a specific backend connection.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("server_id"), Description: "The server ID", Required: true}, {Name: mcp.ParamName("backend_id"), Description: "The backend/connection ID",

		// --- Issues ---
		Required: true}},
	},

	{
		Name:        mcp.ToolName("pganalyze_get_issues"),
		Description: "Get active check-up issues and performance alerts. Shows slow query warnings, index problems, and health issues.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("server_id"), Description: "Server ID to filter issues (optional)"}, {Name: mcp.ParamName("database_id"), Description: "Database ID to filter issues (optional)"}},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_checkup_status"),
		Description: "Get check-up status overview for a database showing passed, warning, and critical checks.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}},
	},
}
