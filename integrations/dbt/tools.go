package dbt

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        mcp.ToolName("dbt_get_job_run_error"),
		Description: "Diagnose a failed or errored dbt Cloud (dbt platform) run: status, git_sha, failing models and tests with error messages, original_file_path, compiled SQL, and the failed step's log tail. Start here for dbt run failures, broken models, and test errors. Returns artifacts_pending=true when artifacts are not ingested yet; wait and retry.",
		Parameters: map[string]string{
			"run_id": "dbt Cloud run ID that failed",
		},
		Required: []string{"run_id"},
	},
	{
		Name:        mcp.ToolName("dbt_get_job_run_regression_range"),
		Description: "Find which git commits introduced a dbt Cloud run failure: returns the last successful run's git_sha (compare_base) and the first failing run's git_sha (compare_head) for the same job. Use after get_job_run_error, then compare the commits on GitHub to find the change or pull request that broke it.",
		Parameters: map[string]string{
			"run_id": "dbt Cloud run ID that failed",
		},
		Required: []string{"run_id"},
	},
	{
		Name:        mcp.ToolName("dbt_list_jobs_runs"),
		Description: "List dbt Cloud job runs newest first, with status, git_sha, and timing. Use to find failed, errored, or successful dbt runs for a job and their run_id.",
		Parameters: map[string]string{
			"job_id": "Optional job ID (job_definition_id) filter",
			"status": "Optional status: queued, starting, running, success, error, cancelled (or numeric code 1, 2, 3, 10, 20, 30)",
			"limit":  "Page size (default 10, max 100)",
			"offset": "Pagination offset (default 0)",
		},
	},
	{
		Name:        mcp.ToolName("dbt_get_job_run_artifact"),
		Description: "Download a dbt Cloud run artifact such as run_results.json, manifest.json, catalog.json, or sources.json, optionally filtered with a jq expression. Large artifacts (manifest.json) require jq, for example '.nodes[\"model.proj.name\"].raw_code'.",
		Parameters: map[string]string{
			"run_id": "dbt Cloud run ID",
			"path":   "Artifact path, e.g. run_results.json, manifest.json, catalog.json, sources.json",
			"step":   "Optional 1-based run step index (default: last step)",
			"jq":     "Optional jq filter applied to the artifact JSON; multiple outputs are returned as an array",
		},
		Required: []string{"run_id", "path"},
	},
	{
		Name:        mcp.ToolName("dbt_get_manifest_nodes"),
		Description: "Map changed dbt files (models, schema yml, macros, seeds, snapshots) or model names to production manifest nodes with materialization. Start here for dbt pull request review and blast radius; files missing from prod are returned as unmatched (new). Macro files include dependent models.",
		Parameters: map[string]string{
			"file_paths": "Repo-relative paths changed in the PR, e.g. [\"models/staging/stg_orders.sql\"]",
			"names":      "Model, seed, or snapshot names to look up",
			"job_id":     "Production job whose latest successful run supplies manifest.json (default: configured prod_job_id)",
			"run_id":     "Use this specific run's manifest.json instead of the latest successful prod run",
		},
	},
	{
		Name:        mcp.ToolName("dbt_get_model_lineage"),
		Description: "Get downstream (children, dependents) or upstream (parents) dbt lineage from the production manifest: impacted models by depth, attached tests, and exposures such as dashboards. Use after get_manifest_nodes for dbt blast radius and impact analysis.",
		Parameters: map[string]string{
			"unique_ids": "Node unique_ids, e.g. [\"model.proj.stg_orders\"]",
			"names":      "Model names (resolved within the root project)",
			"direction":  "down (default, descendants) or up (ancestors)",
			"depth":      "Levels to walk (default 2, matching state:modified+2; 0 = unlimited)",
			"job_id":     "Production job whose latest successful run supplies manifest.json (default: configured prod_job_id)",
			"run_id":     "Use this specific run's manifest.json",
		},
	},
}
