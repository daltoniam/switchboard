package fly

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("fly_list_apps"): {
		"apps[].id", "apps[].name", "apps[].machine_count",
		"apps[].volume_count", "apps[].network",
	},
	mcp.ToolName("fly_list_machines"): {
		"[].id", "[].name", "[].state", "[].region",
		"[].instance_id", "[].private_ip",
		"[].image_ref.repository", "[].image_ref.tag",
		"[].created_at", "[].updated_at",
		"[].config.guest.cpus", "[].config.guest.memory_mb", "[].config.guest.cpu_kind",
	},
	mcp.ToolName("fly_list_volumes"): {
		"[].id", "[].name", "[].state", "[].size_gb",
		"[].region", "[].encrypted", "[].attached_machine_id",
		"[].created_at", "[].auto_backup_enabled",
	},
	mcp.ToolName("fly_list_secrets"): {
		"[].label", "[].type", "[].created_at",
	},
	mcp.ToolName("fly_list_volume_snapshots"): {
		"[].id", "[].size", "[].digest",
		"[].created_at", "[].status", "[].retention_days",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("fly: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
