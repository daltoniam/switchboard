package salesforce

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// ── SObject Describe ────────────────────────────────────────────
	"salesforce_describe_global": {"sobjects[].name", "sobjects[].label", "sobjects[].keyPrefix", "sobjects[].queryable", "sobjects[].createable", "sobjects[].updateable", "sobjects[].deletable", "sobjects[].custom"},
	"salesforce_describe_sobject": {
		"name", "label", "keyPrefix", "queryable", "createable", "updateable", "deletable",
		"fields[].name", "fields[].label", "fields[].type", "fields[].length", "fields[].nillable", "fields[].updateable", "fields[].createable",
		"childRelationships[].childSObject", "childRelationships[].field", "childRelationships[].relationshipName",
	},

	// ── Metadata & Org ──────────────────────────────────────────────
	"salesforce_list_api_versions":    {"[].version", "[].label", "[].url"},
	"salesforce_get_limits":           {},
	"salesforce_list_recently_viewed": {"[].attributes.type", "[].Id", "[].Name"},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		if len(specs) == 0 {
			parsed[tool] = nil
			continue
		}
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("salesforce: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
