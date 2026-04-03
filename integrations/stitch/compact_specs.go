package stitch

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"stitch_list_projects": {
		"projects[].name", "projects[].title",
		"projects[].createTime", "projects[].updateTime",
	},
	"stitch_list_screens": {
		"screens[].name", "screens[].displayName",
		"screens[].createTime", "screens[].updateTime",
	},
	"stitch_list_design_systems": {
		"designSystems[].name", "designSystems[].displayName",
		"designSystems[].createTime",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("stitch: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
