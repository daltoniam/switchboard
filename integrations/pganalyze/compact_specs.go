package pganalyze

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"pganalyze_get_servers": {
		"id", "name", "humanId", "lastSnapshotAt",
		"databases[].id", "databases[].datname", "databases[].displayName",
	},
	"pganalyze_get_issues": {
		"id", "checkGroupAndName", "severity", "state",
		"description", "createdAt", "updatedAt",
		"references[].kind", "references[].name", "references[].url",
	},
	"pganalyze_get_query_stats": {
		"queryId", "truncatedQuery", "queryUrl", "statementTypes",
		"totalCalls", "avgTime", "bufferHitRatio", "pctOfTotal", "callsPerMinute",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("pganalyze: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
