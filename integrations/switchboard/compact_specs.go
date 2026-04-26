package switchboard

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// rawFieldCompactionSpecs maps tool names to dot-notation field compaction specs.
// Only list/read tools get specs — mutation tools return full confirmation responses.
var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("switchboard_list_integrations"): {
		"name", "enabled", "healthy", "tool_count",
	},
	mcp.ToolName("switchboard_check_health"): {
		"name", "healthy", "enabled",
	},
	mcp.ToolName("switchboard_browse_plugins"): {
		"name", "description", "latest_version", "installed", "update_available",
	},
}

// fieldCompactionSpecs holds pre-parsed CompactFields, built once at package init.
var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("switchboard: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
