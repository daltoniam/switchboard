package homeassistant

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("homeassistant_list_states"): {
		"entity_id", "state", "last_changed",
		"attributes.friendly_name", "attributes.unit_of_measurement", "attributes.device_class",
	},
	mcp.ToolName("homeassistant_list_services"): {
		"domain", "services",
	},
	mcp.ToolName("homeassistant_list_events"): {
		"event", "listener_count",
	},
	mcp.ToolName("homeassistant_list_calendars"): {
		"entity_id", "name",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("homeassistant: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
