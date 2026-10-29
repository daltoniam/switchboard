package pganalyze

import mcp "github.com/daltoniam/switchboard"

// staticTools mirrors the pganalyze MCP server's tools/list (names, camelCase
// parameters, required fields). It backs search and execute until the proxy
// handshake succeeds, and seeds the search index built at daemon start, so it
// must track upstream: arguments are passed through to pganalyze verbatim.
var staticTools = []mcp.ToolDefinition{
	// --- Servers ---
	{
		Name:        mcp.ToolName("pganalyze_list_servers"),
		Description: "List monitored PostgreSQL servers in pganalyze. Start here to discover server IDs and database IDs needed by other tools.",
		Parameters: map[string]string{
			"systemId":       "Filter by API system ID set by the collector",
			"systemScope":    "Filter by API system scope set by the collector",
			"systemType":     "Filter by API system type set by the collector",
			"withRecentData": "Filter for servers that have received data in the last 24 hours. If false, filters for servers that haven't received data recently.",
		},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_server_details"),
		Description: "Get details for a specific monitored PostgreSQL server including configuration and snapshot info.",
		Parameters: map[string]string{
			"databaseId": "Database ID to look up the server for (either this or serverId is required)",
			"serverId":   "Server ID (not the server name). Use list_servers to find the serverId. Either this or databaseId is required.",
		},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_postgres_settings"),
		Description: "Get PostgreSQL configuration settings (e.g. shared_buffers, work_mem) for a server.",
		Parameters: map[string]string{
			"databaseId": "Database ID to look up settings for (either this or serverId is required)",
			"patterns":   "Filter settings by name patterns (e.g. ['autovacuum', 'work_mem'])",
			"serverId":   "Server ID to get settings for (either this or databaseId is required)",
		},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_system_stats"),
		Description: "Get time-series operating system metrics for a server over a time range: CPU utilization and load average, memory usage, storage space and I/O, network throughput, and WAL size.",
		Parameters: map[string]string{
			"endTs":    "End Unix timestamp in seconds",
			"metrics":  "Metric groups to return, defaulting to cpu and memory",
			"serverId": "Server ID (from list_servers) to get system metrics for",
			"startTs":  "Start Unix timestamp in seconds",
		},
		Required: []string{"serverId", "startTs", "endTs"},
	},

	// --- Databases ---
	{
		Name:        mcp.ToolName("pganalyze_get_databases"),
		Description: "List databases with size stats and issue counts for a server.",
		Parameters: map[string]string{
			"hidden":   "Include hidden databases",
			"serverId": "Server ID (not the server name). Use list_servers to find the serverId.",
		},
	},

	// --- Queries ---
	{
		Name:        mcp.ToolName("pganalyze_get_query_stats"),
		Description: "Get top queries by runtime percentage. Shows expensive and slow query bottlenecks sorted by impact.",
		Parameters: map[string]string{
			"databaseId":          "Database ID for which to retrieve query statistics",
			"endTs":               "End Unix timestamp in seconds (defaults to now)",
			"limit":               "Maximum number of queries to return",
			"offset":              "Pagination offset",
			"sortBy":              "Field to sort by (defaults to pctOfTotal)",
			"sortDirection":       "Sort direction (defaults to DESC)",
			"startTs":             "Start Unix timestamp in seconds (defaults to 24 hours ago)",
			"statementTypes":      "Filter by statement type. Values: 'read' (SELECT), 'write' (INSERT/UPDATE/DELETE), 'other' (COPY, DDL, etc.). Defaults to all types.",
			"withNormalizedQuery": "Include the full normalized query text (default: false, only truncated query is returned)",
		},
		Required: []string{"databaseId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_details"),
		Description: "Get the full normalized query text for a specific query.",
		Parameters: map[string]string{
			"databaseId": "Database ID the query belongs to",
			"queryId":    "Query ID to get details for",
		},
		Required: []string{"databaseId", "queryId"},
	},

	// --- Tables ---
	{
		Name:        mcp.ToolName("pganalyze_get_tables"),
		Description: "List tables with filtering and pagination for a database.",
		Parameters: map[string]string{
			"databaseId":        "Database ID to list tables for",
			"filter":            "Filter tables by name",
			"includePartitions": "Include partition tables",
			"limit":             "Maximum number of tables to return",
			"offset":            "Pagination offset",
			"parentTableId":     "Filter by parent table ID (for partitions)",
			"sortBy":            "Field to sort by: sizeBytes (data size), indexSizeBytes, toastSizeBytes, liveTuples, deadTupleRatio (bloat indicator), sizeBytesTrend (growth over 7 days), t...",
			"sortDirection":     "Sort direction",
		},
		Required: []string{"databaseId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_table"),
		Description: "Get detailed information about a single table: schema details, columns with per-column stats, indexes, and constraints.",
		Parameters: map[string]string{
			"databaseId":    "Database ID the table belongs to",
			"schemaTableId": "Schema table ID to get details for",
		},
		Required: []string{"databaseId", "schemaTableId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_table_stats"),
		Description: "Get time-series table statistics (row counts, dead tuples, sequential scans, etc.).",
		Parameters: map[string]string{
			"databaseId":    "Database ID the table belongs to",
			"endTs":         "End Unix timestamp in seconds",
			"schemaTableId": "Schema table ID to get stats for",
			"startTs":       "Start Unix timestamp in seconds",
		},
		Required: []string{"databaseId", "schemaTableId", "startTs", "endTs"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_index_selection"),
		Description: "Get Index Advisor results for an existing indexing engine run (runId from a missing index issue). Shows recommended indexes.",
		Parameters: map[string]string{
			"runId": "Indexing engine run ID (from a missing index issue's detailsJson.indexing_engine_run)",
		},
		Required: []string{"runId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_run_index_selection"),
		Description: "Run the Index Advisor for a table to get index recommendations.",
		Parameters: map[string]string{
			"databaseId":    "Database ID",
			"schemaTableId": "Schema table ID",
		},
		Required: []string{"databaseId", "schemaTableId"},
	},

	// --- EXPLAIN Plans ---
	{
		Name:        mcp.ToolName("pganalyze_get_query_explains"),
		Description: "List EXPLAIN plans for a query (last 7 days). Use to find query plan changes and regressions.",
		Parameters: map[string]string{
			"databaseId": "Database ID the query belongs to",
			"endTs":      "End Unix timestamp in seconds (defaults to now)",
			"queryId":    "Query ID to list EXPLAIN plans for",
			"startTs":    "Start Unix timestamp in seconds (oldest: 7 days ago)",
		},
		Required: []string{"databaseId", "queryId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_explain"),
		Description: "Get a specific EXPLAIN plan with full output including node details and costs.",
		Parameters: map[string]string{
			"databaseId": "Database ID the EXPLAIN plan belongs to",
			"explainId":  "EXPLAIN plan ID to retrieve",
		},
		Required: []string{"databaseId", "explainId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_query_explain_from_trace"),
		Description: "Resolve a db.postgresql.plan trace span attribute (server, database, query fingerprint, sample timestamp) to the EXPLAIN plan of that slow execution.",
		Parameters: map[string]string{
			"datname":     "Database name",
			"fingerprint": "Query fingerprint in hex format",
			"serverId":    "Server ID",
			"timestamp":   "Unix timestamp in seconds when the query sample occurred",
		},
		Required: []string{"serverId", "datname", "fingerprint", "timestamp"},
	},

	// --- Backends ---
	{
		Name:        mcp.ToolName("pganalyze_get_backends"),
		Description: "Get a point-in-time snapshot of active connections (backends) and their states for a database.",
		Parameters: map[string]string{
			"databaseId": "Database ID to get backends for",
			"timestamp":  "Unix timestamp in seconds for the point-in-time snapshot",
		},
		Required: []string{"databaseId", "timestamp"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_backend_counts"),
		Description: "Get time-series connection counts by state and lock status (active, idle, idle in transaction, waiting). Use to spot connection spikes and blocking.",
		Parameters: map[string]string{
			"databaseId": "Database ID to get backend counts for (either this or serverId is required)",
			"endTs":      "End Unix timestamp in seconds",
			"serverId":   "Server ID to get backend counts for (either this or databaseId is required)",
			"startTs":    "Start Unix timestamp in seconds",
		},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_backend_details"),
		Description: "Get details for a specific backend connection.",
		Parameters: map[string]string{
			"backendLocator": "The backend ID returned by get_backends",
			"databaseId":     "Database ID the backend belongs to",
		},
		Required: []string{"databaseId", "backendLocator"},
	},

	// --- Issues ---
	{
		Name:        mcp.ToolName("pganalyze_get_issues"),
		Description: "Get active check-up issues and performance alerts. Shows slow query warnings, index problems, and health issues.",
		Parameters: map[string]string{
			"checkGroupsAndNames": "Filter by check type using \"group/name\" format, e.g. [\"index_advisor/indexing_engine\", \"vacuum/txid_wraparound\"]",
			"databaseId":          "Database ID for which to retrieve issues (either this or serverId is required)",
			"includeActivity":     "Include activity history for each issue (e.g. when it was triggered, notifications sent)",
			"includeDetails":      "Include issue-specific metadata for each issue. Avoid when fetching broad server-wide issues to keep response size manageable.",
			"serverId":            "Server ID for which to retrieve issues (either this or databaseId is required)",
			"severity":            "Only return issues with matching severities",
		},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_checkup_status"),
		Description: "Get check-up status overview for a database showing passed, warning, and critical checks.",
		Parameters: map[string]string{
			"databaseId": "Database ID to get check-up status for",
		},
		Required: []string{"databaseId"},
	},

	// --- Logs ---
	{
		Name:        mcp.ToolName("pganalyze_get_log_stats"),
		Description: "Get time-series counts of collected log lines, grouped by time bucket and log classification, over a time range.",
		Parameters: map[string]string{
			"databaseId": "Optional database ID to scope the stats to a single database on the server",
			"endTs":      "End Unix timestamp in seconds",
			"serverId":   "Server ID (from list_servers) to get log stats for",
			"startTs":    "Start Unix timestamp in seconds",
		},
		Required: []string{"serverId", "startTs", "endTs"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_log_lines"),
		Description: "Get individual Postgres log lines for a server, newest first, with optional filters.",
		Parameters: map[string]string{
			"backendPid":       "Filter by backend process ID (PID)",
			"classification":   "Filter by log classification code (e.g. \"A60\", \"W12\"), as returned by get_log_stats",
			"databaseId":       "Optional database ID to filter to a single database on the server",
			"occurredAfter":    "Only include lines at or after this Unix timestamp in seconds",
			"occurredBefore":   "Only include lines at or before this Unix timestamp in seconds",
			"postgresRoleId":   "Filter by Postgres role (user) ID",
			"queryFingerprint": "Filter to lines for a specific query, using the query's fingerprint from get_query_details",
			"serverId":         "Server ID (from list_servers) to get log lines for",
		},
		Required: []string{"serverId"},
	},

	// --- Query Advisor Workbooks ---
	{
		Name:        mcp.ToolName("pganalyze_create_workbook"),
		Description: "Create a new workbook with a baseline query.",
		Parameters: map[string]string{
			"databaseId": "Database ID to create the workbook in",
			"name":       "Workbook name (e.g. \"Optimize slow user lookup\")",
			"queryText":  "The query text. Literal values OR $N placeholders.",
		},
		Required: []string{"databaseId", "name", "queryText"},
	},
	{
		Name:        mcp.ToolName("pganalyze_create_workbook_variant"),
		Description: "Create a workbook variant from a queryText (rewrite) and/or plannerSettings, then run EXPLAIN.",
		Parameters: map[string]string{
			"appliedInsights":      "Optional.",
			"baselineVariantId":    "Baseline variant ID this is derived from",
			"databaseId":           "Database ID",
			"description":          "Optional description",
			"introducedParameters": "Optional.",
			"name":                 "Variant name (e.g. \"CTE materialization rewrite\")",
			"plannerSettings":      "PostgreSQL planner settings as a flat object, e.g. {\"work_mem\": \"256MB\", \"random_page_cost\": \"1.1\"}. Values are strings.",
			"queryText":            "The rewrite using $alias-style parameters from the baseline.",
			"workbookId":           "Workbook ID to add the variant to",
		},
		Required: []string{"databaseId", "workbookId", "baselineVariantId", "queryText"},
	},
	{
		Name:        mcp.ToolName("pganalyze_create_workbook_parameter_set"),
		Description: "Add a parameter set to a workbook and run EXPLAIN on it.",
		Parameters: map[string]string{
			"parameterSet": "Object mapping each parameter name to its value, e.g. { \"$store_id\": 30, \"$status_id\": 1 }",
			"workbookId":   "Workbook ID returned by create_workbook",
		},
		Required: []string{"workbookId", "parameterSet"},
	},
	{
		Name:        mcp.ToolName("pganalyze_update_workbook"),
		Description: "Update a workbook's name, description and/or parameter types.",
		Parameters: map[string]string{
			"description":    "New workbook description/note. Omit to leave it unchanged.",
			"name":           "New workbook name. Omit to leave it unchanged.",
			"parameterTypes": "Pins the Postgres type of a parameter, e.g.",
			"workbookId":     "Workbook ID to update",
		},
		Required: []string{"workbookId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_workbook_details"),
		Description: "Get a workbook overview: baseline query and all variants with their EXPLAIN results and Query Advisor insights.",
		Parameters: map[string]string{
			"workbookId": "Workbook ID to retrieve",
		},
		Required: []string{"workbookId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_sync_workbook"),
		Description: "Sync and return the workbook with fresh EXPLAIN results for every variant.",
		Parameters: map[string]string{
			"rerunExplain": "Re-run EXPLAIN for all variants even if results already exist, e.g. to retry after a failed run. Default false.",
			"workbookId":   "Workbook ID to sync",
		},
		Required: []string{"workbookId"},
	},
	{
		Name:        mcp.ToolName("pganalyze_get_insight_rewrite"),
		Description: "Generate a rewritten query from a Query Advisor insight on the baseline.",
		Parameters: map[string]string{
			"baselineVariantId": "Baseline variant ID to rewrite from",
			"insight":           "Insight JSON string (one entry from a workbookResult nodeInsights array)",
			"rewriteType":       "Insight type to apply",
		},
		Required: []string{"baselineVariantId", "rewriteType", "insight"},
	},
	{
		Name:        mcp.ToolName("pganalyze_delete_workbook_item"),
		Description: "Delete a workbook, one of its variants, or one of its parameter sets.",
		Parameters: map[string]string{
			"parameterSetId": "For target \"parameter_set\": the id of the parameter set to remove (parameterSets[].id from a workbook response, e.g. sync_workbook or get_workbook_details)",
			"target":         "What to delete: the whole workbook, a variant, or a parameter set",
			"variantId":      "For target \"variant\": the id of the variant to delete (not the baseline).",
			"workbookId":     "Workbook ID the item belongs to",
		},
		Required: []string{"workbookId", "target"},
	},
}
