package projectinterop

import mcp "github.com/daltoniam/switchboard"

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(map[string][]string{
	"projectinterop_list_projects": {"name", "repo", "branch"},
})

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	specs := make(map[string][]mcp.CompactField, len(raw))
	for toolName, fields := range raw {
		parsed, err := mcp.ParseCompactSpecs(fields)
		if err != nil {
			panic(err)
		}
		specs[toolName] = parsed
	}
	return specs
}
