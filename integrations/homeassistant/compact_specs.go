package homeassistant

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"homeassistant_list_states": {
		"entity_id", "state", "last_changed",
		"attributes.friendly_name", "attributes.unit_of_measurement", "attributes.device_class",
	},
	"homeassistant_list_services": {
		"domain", "services",
	},
	"homeassistant_list_events": {
		"event", "listener_count",
	},
	"homeassistant_list_calendars": {
		"entity_id", "name",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("homeassistant: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
