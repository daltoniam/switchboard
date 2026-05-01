package rwx

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Runs ────────────────────────────────────────────────────────
	// Handler: jsonResult(map{ref, count, runs: [{run_id, status, commit_sha, title, definition_path, url}]})
	mcp.ToolName("rwx_get_recent_runs"): {"ref", "count", "runs[].run_id", "runs[].status", "runs[].commit_sha", "runs[].title", "runs[].definition_path", "runs[].url"},
	// Handler: jsonResult(map{run_id, url, status, completed, duration_seconds, title, branch, commit_sha, definition_path, failed_tasks: [{key, task_id, has_artifacts}], failed_tests, other_problems})
	mcp.ToolName("rwx_get_run_results"): {"run_id", "url", "status", "completed", "duration_seconds", "title", "branch", "commit_sha", "definition_path", "failed_tasks[].key", "failed_tasks[].task_id", "failed_tasks[].has_artifacts", "failed_tests", "other_problems"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("rwx: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
