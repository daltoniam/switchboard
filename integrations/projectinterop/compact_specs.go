package projectinterop

import mcp "github.com/daltoniam/switchboard"

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(map[mcp.ToolName][]string{
	"projectinterop_list_projects": {"name", "repo", "branch"},
})

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	specs := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for toolName, fields := range raw {
		parsed, err := mcp.ParseCompactSpecs(fields)
		if err != nil {
			panic(err)
		}
		specs[toolName] = parsed
	}
	return specs
}
