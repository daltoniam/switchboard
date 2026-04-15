package botidentity

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("botidentity_gh_list_installations"): {
		"id", "app_id", "app_slug",
		"target_type", "target_id",
		"account.login", "account.id", "account.type",
		"permissions",
		"events",
		"suspended_at",
		"created_at", "updated_at",
	},
	mcp.ToolName("botidentity_gh_list_install_repos"): {
		"repositories[].id", "repositories[].name", "repositories[].full_name",
		"repositories[].private", "repositories[].html_url",
	},
	mcp.ToolName("botidentity_gh_list_webhook_deliveries"): {
		"id", "guid", "event", "action",
		"status", "status_code",
		"delivered_at",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("botidentity: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
