package pganalyze

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("pganalyze_get_servers"): {
		"id", "name", "humanId", "lastSnapshotAt",
		"databases[].id", "databases[].datname", "databases[].displayName",
	},
	mcp.ToolName("pganalyze_get_issues"): {
		"id", "checkGroupAndName", "severity", "state",
		"description", "createdAt", "updatedAt",
		"references[].kind", "references[].name", "references[].url",
	},
	mcp.ToolName("pganalyze_get_query_stats"): {
		"queryId", "truncatedQuery", "queryUrl", "statementTypes",
		"totalCalls", "avgTime", "bufferHitRatio", "pctOfTotal", "callsPerMinute",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("pganalyze: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
