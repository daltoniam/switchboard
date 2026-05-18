package rwx

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Runs ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_launch_ci_run"),
		Description: "Launch a CI/CD pipeline run using the rwx CLI. Start here to run CI. Executes the specified workflow file (default: .rwx/ci.yml).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("workflow"), Description: "Path to the RWX workflow YAML file to run (default: .rwx/ci.yml, e.g. .rwx/auto-deploy.yml)"}, {Name: mcp.ParamName("targets"), Description: "JSON array of specific task keys to target (optional)"}, {Name: mcp.ParamName("wait"), Description: "Wait for the run to complete before returning (true/false, default: false)"}, {Name: mcp.ParamName("title"), Description: "Display title for the run in the RWX UI (optional)"}, {Name: mcp.ParamName("init"), Description: `JSON object of init parameters available in the init context (optional, e.g. {"deploy_env": "staging"})`}},
	},
	{
		Name:        mcp.ToolName("rwx_dispatch_run"),
		Description: "Launch a run from a pre-configured RWX dispatch workflow by key. Use instead of rwx_launch_ci_run when triggering remote/pre-configured workflows without local files.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("dispatch_key"), Description: "The dispatch key identifying the pre-configured workflow", Required: true}, {Name: mcp.ParamName("ref"), Description: "Git ref (branch/tag/SHA) to use for the run (optional)"}, {Name: mcp.ParamName("params"), Description: "JSON object of dispatch params available in event.dispatch.params context (optional)"}, {Name: mcp.ParamName("wait"), Description: "Wait for the run to complete before returning (true/false, default: false)"}, {Name: mcp.ParamName("title"), Description: "Display title for the run in the RWX UI (optional)"}},
	},
	{
		Name:        mcp.ToolName("rwx_wait_for_ci_run"),
		Description: "Poll and wait for an RWX CI run to complete or timeout",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("run_id"), Description: "RWX run ID or full URL to wait for", Required: true}, {Name: mcp.ParamName("timeout_seconds"), Description: "Maximum time to wait in seconds (default: 1800)"}, {Name: mcp.ParamName("poll_interval_seconds"), Description: "Seconds between status checks (default: 30)"}},
	},
	{
		Name:        mcp.ToolName("rwx_get_recent_runs"),
		Description: "Get recent CI/CD runs for a git branch. Returns all workflow runs by default; use definition_path to filter to a specific workflow.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("ref"), Description: "Git ref (branch name) to filter runs by", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of runs to return (default: 5)"}, {Name: mcp.ParamName("definition_path"), Description: "Filter to a specific workflow file (e.g. .rwx/ci.yml, .rwx/auto-deploy.yml). Omit to return all workflows."}},
	},
	{
		Name:        mcp.ToolName("rwx_get_run_results"),
		Description: "Get structured CI/CD pipeline results including per-task pass/fail status, failed tests, and build errors. Accepts a run ID or branch/commit lookup.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("run_id"), Description: "RWX run ID or full URL (required unless branch is provided)"}, {Name: mcp.ParamName("task_key"), Description: "Get results for a specific task by key (e.g. ci.checks.lint) instead of the full run"}, {Name: mcp.ParamName("branch"), Description: "Look up results by branch name instead of run ID (uses current repo unless repo is set)"}, {Name: mcp.ParamName("commit"), Description: "Look up results by commit SHA instead of run ID"}, {Name: mcp.ParamName("repo"), Description: "Repository name for branch/commit lookup (default: current git repo)"},

		// ── Logs ────────────────────────────────────────────────────────
		{Name: mcp.ParamName("definition"), Description: "Definition path for branch/commit lookup (e.g. .rwx/ci.yml)"}},
	},

	{
		Name:        mcp.ToolName("rwx_get_task_logs"),
		Description: "Download and return full CI/CD task logs with build failure and test failure highlights",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("task_id"), Description: "RWX task ID (32-char hex) or task URL"}, {Name: mcp.ParamName("run_id"), Description: "RWX run ID — use with task_key to resolve by key instead of ID (optional)"}, {Name: mcp.ParamName("task_key"), Description: "Task key (e.g. ci.checks.lint) — requires run_id. Use instead of task_id."}},
	},
	{
		Name:        mcp.ToolName("rwx_head_logs"),
		Description: "Return the first N lines of logs for a run or task. Supports pagination via offset.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "RWX run ID or task ID", Required: true}, {Name: mcp.ParamName("lines"), Description: "Number of lines to return from the beginning (default: 50, max: 50)"}, {Name: mcp.ParamName("offset"), Description: "Line offset to start from (default: 0). Use for pagination."}, {Name: mcp.ParamName("task_key"), Description: "Task key (e.g. ci.checks.lint) — when set, id is treated as run ID and task is resolved by key"}},
	},
	{
		Name:        mcp.ToolName("rwx_tail_logs"),
		Description: "Return the last N lines of logs for a run or task. Supports pagination via offset.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "RWX run ID or task ID", Required: true}, {Name: mcp.ParamName("lines"), Description: "Number of lines to return from the end (default: 50, max: 50)"}, {Name: mcp.ParamName("offset"), Description: "Line offset from the end (default: 0). Use for pagination to see earlier lines."}, {Name: mcp.ParamName("task_key"), Description: "Task key (e.g. ci.checks.lint) — when set, id is treated as run ID and task is resolved by key"}},
	},
	{
		Name:        mcp.ToolName("rwx_grep_logs"),
		Description: "Search CI/CD build and test logs for a pattern with context lines. Results are paginated (50 lines per page).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "RWX run ID or task ID", Required: true}, {Name: mcp.ParamName("pattern"), Description: "Pattern to search for in the logs (case-insensitive)", Required: true}, {Name: mcp.ParamName("context"), Description: "Number of context lines before and after matches (default: 3)"}, {Name: mcp.ParamName("page"), Description: "Page number (default: 1). Each page returns up to 50 lines of output."}, {Name: mcp.ParamName("task_key"), Description:

		// ── Artifacts ───────────────────────────────────────────────────
		"Task key (e.g. ci.checks.lint) — when set, id is treated as run ID and task is resolved by key"}},
	},

	{
		Name:        mcp.ToolName("rwx_get_artifacts"),
		Description: "List or download artifacts for a run",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("run_id"), Description: "RWX run ID or full URL to get artifacts for", Required: true}, {Name: mcp.ParamName("download"), Description: "Download artifacts (true/false, default: false — just list)"}, {Name: mcp.ParamName("artifact_key"), Description: "Specific artifact key to download (optional, downloads all if not specified)"}},
	},

	// ── Workflow ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_validate_workflow"),
		Description: "Validate an RWX workflow YAML file using the rwx CLI",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("file_path"), Description: "Path to the RWX workflow YAML file to validate (default: .rwx/ci.yml)"}},
	},

	// ── Docs ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_docs_search"),
		Description: "Search RWX documentation. Use to find docs on caching, parallelism, configuration, and other RWX features.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (e.g. 'caching', 'parallelism', 'filtering files')", Required: true}, {Name: mcp.ParamName("limit"), Description: "Maximum number of results (default: 5)"}},
	},
	{
		Name:        mcp.ToolName("rwx_docs_pull"),
		Description: "Fetch an RWX documentation article as markdown. Use a URL from rwx_docs_search results or a docs path like /docs/caching.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("url_or_path"), Description: "Full URL (https://www.rwx.com/docs/...) or path (/docs/caching) of the article to fetch", Required: true}},
	},

	// ── Vaults ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_vaults_var_show"),
		Description: "Show a variable value from an RWX vault",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Variable name to show", Required: true}, {Name: mcp.ParamName("vault"), Description: "Vault name (default: 'default')"}},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_var_set"),
		Description: "Set a variable in an RWX vault",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Variable name", Required: true}, {Name: mcp.ParamName("value"), Description: "Variable value", Required: true}, {Name: mcp.ParamName("vault"), Description: "Vault name (default: 'default')"}},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_var_delete"),
		Description: "Delete a variable from an RWX vault",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Variable name to delete", Required: true}, {Name: mcp.ParamName("vault"), Description: "Vault name (default: 'default')"}},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_secret_set"),
		Description: "Set a secret in an RWX vault. The value is stored encrypted and cannot be read back.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Secret name", Required: true}, {Name: mcp.ParamName("value"), Description: "Secret value", Required: true}, {Name: mcp.ParamName("vault"), Description: "Vault name (default: 'default')"}},
	},
	{
		Name:        mcp.ToolName("rwx_vaults_secret_delete"),
		Description: "Delete a secret from an RWX vault",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Secret name to delete", Required: true}, {Name: mcp.ParamName("vault"), Description: "Vault name (default: 'default')"}},
	},

	// ── CLI ─────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("rwx_verify_cli"),
		Description: "Verify the rwx CLI is installed and meets the minimum version requirement (>= " + minRWXVersion + ")",
		Parameters:  []mcp.Parameter{},
	},
}
