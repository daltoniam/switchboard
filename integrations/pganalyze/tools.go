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
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_server_details"),
		Description: "Get details for a specific monitored PostgreSQL server including configuration and snapshot info.",
		Parameters: map[string]string{
			"server_id": "The server ID (from pganalyze_list_servers)",
		},
		Required: []string{"server_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_postgres_settings"),
		Description: "Get PostgreSQL configuration settings (e.g. shared_buffers, work_mem) for a server.",
		Parameters: map[string]string{
			"server_id": "The server ID (from pganalyze_list_servers)",
		},
		Required: []string{"server_id"},
	},

	// --- Databases ---
	{
		Name:        mcp.ToolName("pganalyze_get_databases"),
		Description: "List databases with size stats and issue counts for a server.",
		Parameters: map[string]string{
			"server_id": "The server ID (from pganalyze_list_servers)",
		},
		Required: []string{"server_id"},
	},

	// --- Queries ---
	{
		Name:        mcp.ToolName("pganalyze_get_query_stats"),
		Description: "Get top queries by runtime percentage. Shows expensive and slow query bottlenecks sorted by impact.",
		Parameters: map[string]string{
			"database_id": "Database ID (from pganalyze_list_servers or pganalyze_get_databases)",
			"limit":       "Number of queries to return (default: 10)",
		},
		Required: []string{"database_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_details"),
		Description: "Get the full normalized query text for a specific query.",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"query_id":    "Query ID (from pganalyze_get_query_stats)",
		},
		Required: []string{"database_id", "query_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_samples"),
		Description: "Get sample executions for a query with runtime and parameters.",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"query_id":    "Query ID (from pganalyze_get_query_stats)",
		},
		Required: []string{"database_id", "query_id"},
	},

	// --- Tables ---
	{
		Name:        mcp.ToolName("pganalyze_get_tables"),
		Description: "List tables with filtering and pagination for a database.",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"schema_name": "Filter by schema name (optional)",
			"limit":       "Number of tables to return (optional)",
		},
		Required: []string{"database_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_table"),
		Description: "Get detailed information about a single table: schema details, columns with per-column stats, indexes, and constraints.",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"schema_name": "Schema name",
			"table_name":  "Table name",
		},
		Required: []string{"database_id", "schema_name", "table_name"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_table_stats"),
		Description: "Get time-series table statistics (row counts, dead tuples, sequential scans, etc.).",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"schema_name": "Schema name",
			"table_name":  "Table name",
		},
		Required: []string{"database_id", "schema_name", "table_name"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_index_selection"),
		Description: "Get Index Advisor results for an existing run. Shows recommended indexes.",
		Parameters: map[string]string{
			"database_id": "Database ID",
		},
		Required: []string{"database_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_run_index_selection"),
		Description: "Run the Index Advisor for a table to get index recommendations.",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"schema_name": "Schema name",
			"table_name":  "Table name",
		},
		Required: []string{"database_id", "schema_name", "table_name"},
	},

	// --- EXPLAIN Plans ---
	{
		Name:        mcp.ToolName("pganalyze_get_query_explains"),
		Description: "List EXPLAIN plans for a query (last 7 days). Use to find query plan changes and regressions.",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"query_id":    "Query ID (from pganalyze_get_query_stats)",
		},
		Required: []string{"database_id", "query_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_explain"),
		Description: "Get a specific EXPLAIN plan with full output including node details and costs.",
		Parameters: map[string]string{
			"database_id": "Database ID",
			"explain_id":  "EXPLAIN plan ID (from pganalyze_get_query_explains)",
		},
		Required: []string{"database_id", "explain_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_explain_from_trace"),
		Description: "Resolve an OpenTelemetry trace span to an EXPLAIN plan. Requires OpenTelemetry integration.",
		Parameters: map[string]string{
			"trace_id": "OpenTelemetry trace ID",
			"span_id":  "OpenTelemetry span ID",
		},
		Required: []string{"trace_id", "span_id"},
	},

	// --- Backends ---
	{
		Name:        mcp.ToolName("pganalyze_get_backend_counts"),
		Description: "Get time-series connection counts by state (active, idle, waiting).",
		Parameters: map[string]string{
			"server_id": "The server ID",
		},
		Required: []string{"server_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_backends"),
		Description: "Get a point-in-time snapshot of active connections and their states.",
		Parameters: map[string]string{
			"server_id": "The server ID",
		},
		Required: []string{"server_id"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_backend_details"),
		Description: "Get details for a specific backend connection.",
		Parameters: map[string]string{
			"server_id":  "The server ID",
			"backend_id": "The backend/connection ID",
		},
		Required: []string{"server_id", "backend_id"},
	},

	// --- Issues ---
	{
		Name:        mcp.ToolName("pganalyze_get_issues"),
		Description: "Get active check-up issues and performance alerts. Shows slow query warnings, index problems, and health issues.",
		Parameters: map[string]string{
			"server_id":   "Server ID to filter issues (optional)",
			"database_id": "Database ID to filter issues (optional)",
		},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_checkup_status"),
		Description: "Get check-up status overview for a database showing passed, warning, and critical checks.",
		Parameters: map[string]string{
			"database_id": "Database ID",
		},
		Required: []string{"database_id"},
	},
}
