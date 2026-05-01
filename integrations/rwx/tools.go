package rwx

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Runs ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_launch_ci_run"),
		Description: "Launch a CI/CD pipeline run using the rwx CLI. Start here to run CI. Executes the specified workflow file (default: .rwx/ci.yml).",
		Parameters: map[string]string{
			"workflow": "Path to the RWX workflow YAML file to run (default: .rwx/ci.yml, e.g. .rwx/auto-deploy.yml)",
			"targets":  "JSON array of specific task keys to target (optional)",
			"wait":     "Wait for the run to complete before returning (true/false, default: false)",
			"title":    "Display title for the run in the RWX UI (optional)",
			"init":     "JSON object of init parameters available in the init context (optional, e.g. {\"deploy_env\": \"staging\"})",
		},
	},
	{
		Name:        mcp.ToolName("rwx_dispatch_run"),
		Description: "Launch a run from a pre-configured RWX dispatch workflow by key. Use instead of rwx_launch_ci_run when triggering remote/pre-configured workflows without local files.",
		Parameters: map[string]string{
			"dispatch_key": "The dispatch key identifying the pre-configured workflow",
			"ref":          "Git ref (branch/tag/SHA) to use for the run (optional)",
			"params":       "JSON object of dispatch params available in event.dispatch.params context (optional)",
			"wait":         "Wait for the run to complete before returning (true/false, default: false)",
			"title":        "Display title for the run in the RWX UI (optional)",
		},
		Required: []string{"dispatch_key"},
	},
	{
		Name:        mcp.ToolName("rwx_wait_for_ci_run"),
		Description: "Poll and wait for an RWX CI run to complete or timeout",
		Parameters: map[string]string{
			"run_id":                "RWX run ID or full URL to wait for",
			"timeout_seconds":       "Maximum time to wait in seconds (default: 1800)",
			"poll_interval_seconds": "Seconds between status checks (default: 30)",
		},
		Required: []string{"run_id"},
	},
	{
		Name:        mcp.ToolName("rwx_get_recent_runs"),
		Description: "Get recent CI/CD runs for a git branch. Returns all workflow runs by default; use definition_path to filter to a specific workflow.",
		Parameters: map[string]string{
			"ref":             "Git ref (branch name) to filter runs by",
			"limit":           "Number of runs to return (default: 5)",
			"definition_path": "Filter to a specific workflow file (e.g. .rwx/ci.yml, .rwx/auto-deploy.yml). Omit to return all workflows.",
		},
		Required: []string{"ref"},
	},
	{
		Name:        mcp.ToolName("rwx_get_run_results"),
		Description: "Get structured CI/CD pipeline results including per-task pass/fail status, failed tests, and build errors. Accepts a run ID or branch/commit lookup.",
		Parameters: map[string]string{
			"run_id":     "RWX run ID or full URL (required unless branch is provided)",
			"task_key":   "Get results for a specific task by key (e.g. ci.checks.lint) instead of the full run",
			"branch":     "Look up results by branch name instead of run ID (uses current repo unless repo is set)",
			"commit":     "Look up results by commit SHA instead of run ID",
			"repo":       "Repository name for branch/commit lookup (default: current git repo)",
			"definition": "Definition path for branch/commit lookup (e.g. .rwx/ci.yml)",
		},
	},

	// ── Logs ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_get_task_logs"),
		Description: "Download and return full CI/CD task logs with build failure and test failure highlights",
		Parameters: map[string]string{
			"task_id":  "RWX task ID (32-char hex) or task URL",
			"run_id":   "RWX run ID — use with task_key to resolve by key instead of ID (optional)",
			"task_key": "Task key (e.g. ci.checks.lint) — requires run_id. Use instead of task_id.",
		},
	},
	{
		Name:        mcp.ToolName("rwx_head_logs"),
		Description: "Return the first N lines of logs for a run or task. Supports pagination via offset.",
		Parameters: map[string]string{
			"id":       "RWX run ID or task ID",
			"lines":    "Number of lines to return from the beginning (default: 50, max: 50)",
			"offset":   "Line offset to start from (default: 0). Use for pagination.",
			"task_key": "Task key (e.g. ci.checks.lint) — when set, id is treated as run ID and task is resolved by key",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("rwx_tail_logs"),
		Description: "Return the last N lines of logs for a run or task. Supports pagination via offset.",
		Parameters: map[string]string{
			"id":       "RWX run ID or task ID",
			"lines":    "Number of lines to return from the end (default: 50, max: 50)",
			"offset":   "Line offset from the end (default: 0). Use for pagination to see earlier lines.",
			"task_key": "Task key (e.g. ci.checks.lint) — when set, id is treated as run ID and task is resolved by key",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("rwx_grep_logs"),
		Description: "Search CI/CD build and test logs for a pattern with context lines. Results are paginated (50 lines per page).",
		Parameters: map[string]string{
			"id":       "RWX run ID or task ID",
			"pattern":  "Pattern to search for in the logs (case-insensitive)",
			"context":  "Number of context lines before and after matches (default: 3)",
			"page":     "Page number (default: 1). Each page returns up to 50 lines of output.",
			"task_key": "Task key (e.g. ci.checks.lint) — when set, id is treated as run ID and task is resolved by key",
		},
		Required: []string{"id", "pattern"},
	},

	// ── Artifacts ───────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_get_artifacts"),
		Description: "List or download artifacts for a run",
		Parameters: map[string]string{
			"run_id":       "RWX run ID or full URL to get artifacts for",
			"download":     "Download artifacts (true/false, default: false — just list)",
			"artifact_key": "Specific artifact key to download (optional, downloads all if not specified)",
		},
		Required: []string{"run_id"},
	},

	// ── Workflow ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_validate_workflow"),
		Description: "Validate an RWX workflow YAML file using the rwx CLI",
		Parameters: map[string]string{
			"file_path": "Path to the RWX workflow YAML file to validate (default: .rwx/ci.yml)",
		},
	},

	// ── Docs ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_docs_search"),
		Description: "Search RWX documentation. Use to find docs on caching, parallelism, configuration, and other RWX features.",
		Parameters: map[string]string{
			"query": "Search query (e.g. 'caching', 'parallelism', 'filtering files')",
			"limit": "Maximum number of results (default: 5)",
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("rwx_docs_pull"),
		Description: "Fetch an RWX documentation article as markdown. Use a URL from rwx_docs_search results or a docs path like /docs/caching.",
		Parameters: map[string]string{
			"url_or_path": "Full URL (https://www.rwx.com/docs/...) or path (/docs/caching) of the article to fetch",
		},
		Required: []string{"url_or_path"},
	},

	// ── Vaults ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_vaults_var_show"),
		Description: "Show a variable value from an RWX vault",
		Parameters: map[string]string{
			"name":  "Variable name to show",
			"vault": "Vault name (default: 'default')",
		},
		Required: []string{"name"},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_var_set"),
		Description: "Set a variable in an RWX vault",
		Parameters: map[string]string{
			"name":  "Variable name",
			"value": "Variable value",
			"vault": "Vault name (default: 'default')",
		},
		Required: []string{"name", "value"},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_var_delete"),
		Description: "Delete a variable from an RWX vault",
		Parameters: map[string]string{
			"name":  "Variable name to delete",
			"vault": "Vault name (default: 'default')",
		},
		Required: []string{"name"},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_secret_set"),
		Description: "Set a secret in an RWX vault. The value is stored encrypted and cannot be read back.",
		Parameters: map[string]string{
			"name":  "Secret name",
			"value": "Secret value",
			"vault": "Vault name (default: 'default')",
		},
		Required: []string{"name", "value"},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_secret_delete"),
		Description: "Delete a secret from an RWX vault",
		Parameters: map[string]string{
			"name":  "Secret name to delete",
			"vault": "Vault name (default: 'default')",
		},
		Required: []string{"name"},
	},

	// ── CLI ─────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_verify_cli"),
		Description: "Verify the rwx CLI is installed and meets the minimum version requirement (>= " + minRWXVersion + ")",
		Parameters:  map[string]string{},
	},
}
