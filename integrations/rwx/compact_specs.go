package rwx

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Runs ────────────────────────────────────────────────────────
	// Handler: jsonResult(map{ref, count, runs: [{run_id, status, commit_sha, title, url}]})
	mcp.ToolName("rwx_get_recent_runs"): {"ref", "count", "runs[].run_id", "runs[].status", "runs[].commit_sha", "runs[].title", "runs[].url"},
	// Handler: jsonResult(map{run_id, url, status, execution, duration_seconds, summary, failed_tasks, tasks: [{key, status, duration_seconds, cache_hit}]})
	mcp.ToolName("rwx_get_run_results"): {"run_id", "url", "status", "duration_seconds", "summary", "failed_tasks", "tasks[].key", "tasks[].status", "tasks[].duration_seconds", "tasks[].cache_hit"},
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
