package cloudflare

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	mcp.ToolName("cloudflare_list_zones"): {
		"result[].id", "result[].name", "result[].status",
		"result[].paused", "result[].type",
		"result[].name_servers",
		"result[].plan.name",
		"result[].activated_on", "result[].modified_on",
	},
	mcp.ToolName("cloudflare_list_dns_records"): {
		"result[].id", "result[].type", "result[].name",
		"result[].content", "result[].ttl", "result[].proxied",
		"result[].priority", "result[].created_on", "result[].modified_on",
	},
	mcp.ToolName("cloudflare_list_workers"): {
		"result[].id", "result[].etag",
		"result[].created_on", "result[].modified_on",
	},
	mcp.ToolName("cloudflare_list_worker_routes"): {
		"result[].id", "result[].pattern", "result[].script",
	},
	mcp.ToolName("cloudflare_list_pages_projects"): {
		"result[].id", "result[].name", "result[].subdomain",
		"result[].created_on", "result[].production_branch",
		"result[].source.type", "result[].source.config.production_branch",
	},
	mcp.ToolName("cloudflare_list_pages_deployments"): {
		"result[].id", "result[].short_id", "result[].environment",
		"result[].url", "result[].created_on",
		"result[].source.type",
		"result[].deployment_trigger.type",
		"result[].deployment_trigger.metadata.branch",
		"result[].deployment_trigger.metadata.commit_hash",
	},
	"cloudflare_list_r2_buckets": {
		"buckets[].name", "buckets[].creation_date", "buckets[].location",
	},
	mcp.ToolName("cloudflare_list_kv_namespaces"): {
		"result[].id", "result[].title", "result[].supports_url_encoding",
	},
	mcp.ToolName("cloudflare_list_kv_keys"): {
		"result[].name", "result[].expiration", "result[].metadata",
	},
	"cloudflare_list_d1_databases": {
		"result[].uuid", "result[].name", "result[].version",
		"result[].num_tables", "result[].file_size",
		"result[].created_at",
	},
	mcp.ToolName("cloudflare_list_waf_rulesets"): {
		"result[].id", "result[].name", "result[].kind",
		"result[].phase", "result[].version",
		"result[].last_updated",
	},
	mcp.ToolName("cloudflare_list_load_balancers"): {
		"result[].id", "result[].name", "result[].enabled",
		"result[].default_pools",
		"result[].fallback_pool",
		"result[].proxied",
		"result[].ttl",
		"result[].modified_on",
	},
	mcp.ToolName("cloudflare_list_lb_pools"): {
		"result[].id", "result[].name", "result[].enabled",
		"result[].healthy", "result[].minimum_origins",
		"result[].origins[].name", "result[].origins[].address",
		"result[].origins[].enabled", "result[].origins[].healthy",
		"result[].modified_on",
	},
	mcp.ToolName("cloudflare_list_lb_monitors"): {
		"result[].id", "result[].type", "result[].description",
		"result[].method", "result[].path",
		"result[].interval", "result[].timeout",
		"result[].expected_codes",
		"result[].modified_on",
	},
	mcp.ToolName("cloudflare_list_accounts"): {
		"result[].id", "result[].name", "result[].type",
		"result[].settings.enforce_twofactor",
		"result[].created_on",
	},
	mcp.ToolName("cloudflare_list_account_members"): {
		"result[].id", "result[].user.email",
		"result[].user.first_name", "result[].user.last_name",
		"result[].status",
		"result[].roles[].name",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("cloudflare: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
