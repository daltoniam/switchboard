package pganalyze

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Servers ---
	{
		Name:        "pganalyze_get_servers",
		Description: "List all monitored PostgreSQL servers and their databases from pganalyze. Start here to discover monitored servers.",
		Parameters: map[string]string{
			"organization_slug": "Organization slug to filter servers (e.g. 'my-org')",
		},
	},

	// --- Issues ---
	{
		Name:        "pganalyze_get_issues",
		Description: "Get check-up issues and performance alerts for monitored databases. Surfaces slow query warnings, index problems, and health issues. Returns open issues by default.",
		Parameters: map[string]string{
			"organization_slug": "Organization slug to filter issues",
			"server_id":         "Server ID to filter issues (from pganalyze_get_servers)",
			"database_id":       "Database ID to filter issues (from pganalyze_get_servers)",
			"severity":          "Filter by severity: info, warning, critical",
			"include_resolved":  "Include resolved issues (default: false)",
		},
	},

	// --- Query Stats ---
	{
		Name:        "pganalyze_get_query_stats",
		Description: "Get query performance statistics for a database, sorted by impact. Shows top queries by total runtime. Use to find slow and expensive query bottlenecks.",
		Parameters: map[string]string{
			"database_id": "Database ID (use pganalyze_get_servers to find this)",
			"start_ts":    "Start Unix timestamp in seconds (defaults to 24 hours ago)",
			"end_ts":      "End Unix timestamp in seconds (defaults to now)",
			"limit":       "Number of queries to return (default: 20)",
		},
		Required: []string{"database_id"},
	},
}
