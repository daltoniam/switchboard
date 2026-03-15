package rwx

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Runs ────────────────────────────────────────────────────────
	{
		Name:        "rwx_launch_ci_run",
		Description: "Launch a CI run using the rwx CLI. Runs .rwx/ci.yml by default.",
		Parameters: map[string]string{
			"targets": "JSON array of specific task keys to target (optional)",
			"wait":    "Wait for the run to complete before returning (true/false, default: false)",
		},
	},
	{
		Name:        "rwx_wait_for_ci_run",
		Description: "Poll and wait for an RWX CI run to complete or timeout",
		Parameters: map[string]string{
			"run_id":                "RWX run ID or full URL to wait for",
			"timeout_seconds":       "Maximum time to wait in seconds (default: 1800)",
			"poll_interval_seconds": "Seconds between status checks (default: 30)",
		},
		Required: []string{"run_id"},
	},
	{
		Name:        "rwx_get_recent_runs",
		Description: "Get recent CI runs for a git branch, filtered to .rwx/ci.yml runs",
		Parameters: map[string]string{
			"ref":   "Git ref (branch name) to filter runs by",
			"limit": "Number of runs to return (default: 5)",
		},
		Required: []string{"ref"},
	},
	{
		Name:        "rwx_get_run_results",
		Description: "Get structured results for a completed run, including per-task status and summary",
		Parameters: map[string]string{
			"run_id": "RWX run ID or full URL to get results for",
		},
		Required: []string{"run_id"},
	},

	// ── Logs ────────────────────────────────────────────────────────
	{
		Name:        "rwx_get_task_logs",
		Description: "Download and return full logs for a task, with failure highlights",
		Parameters: map[string]string{
			"task_id": "RWX task ID (32-char hex) or task URL",
		},
		Required: []string{"task_id"},
	},
	{
		Name:        "rwx_head_logs",
		Description: "Return the first N lines of logs for a run or task. Supports pagination via offset.",
		Parameters: map[string]string{
			"id":     "RWX run ID or task ID",
			"lines":  "Number of lines to return from the beginning (default: 50, max: 50)",
			"offset": "Line offset to start from (default: 0). Use for pagination.",
		},
		Required: []string{"id"},
	},
	{
		Name:        "rwx_tail_logs",
		Description: "Return the last N lines of logs for a run or task. Supports pagination via offset.",
		Parameters: map[string]string{
			"id":     "RWX run ID or task ID",
			"lines":  "Number of lines to return from the end (default: 50, max: 50)",
			"offset": "Line offset from the end (default: 0). Use for pagination to see earlier lines.",
		},
		Required: []string{"id"},
	},
	{
		Name:        "rwx_grep_logs",
		Description: "Search logs for a pattern with context lines. Results are paginated (50 lines per page).",
		Parameters: map[string]string{
			"id":      "RWX run ID or task ID",
			"pattern": "Pattern to search for in the logs (case-insensitive)",
			"context": "Number of context lines before and after matches (default: 3)",
			"page":    "Page number (default: 1). Each page returns up to 50 lines of output.",
		},
		Required: []string{"id", "pattern"},
	},

	// ── Artifacts ───────────────────────────────────────────────────
	{
		Name:        "rwx_get_artifacts",
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
		Name:        "rwx_validate_workflow",
		Description: "Validate an RWX workflow YAML file using the rwx CLI",
		Parameters: map[string]string{
			"file_path": "Path to the RWX workflow YAML file to validate (default: .rwx/ci.yml)",
		},
	},

	// ── CLI ─────────────────────────────────────────────────────────
	{
		Name:        "rwx_verify_cli",
		Description: "Verify the rwx CLI is installed and meets the minimum version requirement (>= " + minRWXVersion + ")",
		Parameters:  map[string]string{},
	},
}
