package aikido

import (
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

var cloudEndpoints = []endpoint{
	{
		tool: "aikido_list_clouds", method: http.MethodGet, path: "/clouds", scope: "clouds:read", access: mcp.AccessRead,
		desc: "List cloud accounts connected to Aikido for cloud security posture management (CSPM): AWS, Azure, GCP and Kubernetes environments with provider, environment, external account id and last scan. Start here for Aikido cloud posture and misconfiguration findings.",
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_list_cloud_assets", method: http.MethodGet, path: "/clouds/assets", scope: "clouds:read", access: mcp.AccessRead,
		desc: "Search the Aikido cloud asset inventory (S3 buckets, EC2 and VM instances, IAM roles, databases and other cloud resources) by text, asset type, region, cloud, provider or source id. Paginated with page and limit; the response has totalCount and hasMore.",
		params: ps(
			str("search", "Free-text search over asset metadata."),
			strs("asset_type", "Only these asset types (list or comma-separated)."),
			strs("region", "Only these regions."),
			ints("cloud_id", "Only these Aikido cloud ids from aikido_list_clouds."),
			strs("provider", "Only these providers.").oneOf("aws", "azure", "gcp", "alibaba", "oci", "supabase", "render"),
			strs("source_id", "Only these provider-specific resource ids."),
			str("sort", "Sort, for example asset_type:asc,region:desc (fields name, asset_type, region, source_id, cloud_id)."),
			boolean("include_metadata", "Include each asset's full metadata object (large)."),
			strs("metadata_fields", "Metadata paths to project into selected_metadata."),
			integer("page", "Zero-based page number (default 0)."),
			integer("limit", "Assets per page."),
		),
	},
	{
		tool: "aikido_list_cloud_rules", method: http.MethodGet, path: "/clouds/rules", scope: "clouds:read", access: mcp.AccessRead,
		desc:   "List Aikido cloud security posture (CSPM) rules with description, how to fix, cloud type and open issue count.",
		params: ps(integer("cloud_id", "Count open issues for this cloud id only.")),
	},
	{
		tool: "aikido_connect_aws_cloud", method: http.MethodPost, path: "/clouds/aws", scope: "clouds:write", access: mcp.AccessWrite,
		desc: "Connect an AWS account to Aikido cloud scanning through an IAM role Aikido can assume (create the role first with Aikido's CloudFormation template).",
		params: ps(
			str("role_arn", "ARN of the IAM role Aikido assumes.").req(),
			str("name", "Display name for the cloud.").req(),
			str("environment", "Environment the account serves.").req().oneOf("production", "staging", "development", "mixed"),
		),
	},
	{
		tool: "aikido_remove_cloud", method: http.MethodDelete, path: "/clouds/{cloud_id}", scope: "clouds:write", access: mcp.AccessDestructive,
		desc:   "Disconnect a cloud account from Aikido, removing its assets and posture findings.",
		params: ps(integer("cloud_id", "Cloud id from aikido_list_clouds.")),
	},
	{
		tool: "aikido_list_virtual_machines", method: http.MethodGet, path: "/virtual-machines", scope: "virtual_machines:read", access: mcp.AccessRead,
		desc: "List virtual machines (cloud VM instances) Aikido scans for OS package vulnerabilities, with cloud, region, environment, open and ignored issue counts and optional OS and version.",
		params: ps(
			integer("cloud_id", "Only VMs in this cloud id."),
			str("search", "Search VM names."),
			str("filter_os", "Only this operating system."),
			str("filter_os_version", "Only this operating system version."),
			boolean("include_os_info", "Include operating system and version.").asInt(),
		),
		page: pages(1, 100, 20),
	},
	{
		tool: "aikido_export_virtual_machine_sbom", method: http.MethodGet, path: "/virtual-machines/{virtual_machine_id}/export/{format}", scope: "virtual_machines:read", access: mcp.AccessRead,
		desc: "Export the SBOM (installed packages and licenses) of one Aikido-scanned virtual machine as CycloneDX JSON, SPDX or CSV.",
		params: ps(
			integer("virtual_machine_id", "Virtual machine id from aikido_list_virtual_machines."),
			str("format", "Export format.").oneOf("sbom", "sbom_spdx", "csv"),
		),
	},
}

var domainEndpoints = []endpoint{
	{
		tool: "aikido_list_domains", method: http.MethodGet, path: "/domains", scope: "domains:read", access: mcp.AccessRead,
		desc: "List domains and APIs monitored by Aikido surface monitoring (DAST, attack surface and API scanning) with kind, scan frequency, last scan and whether authenticated scanning is set up. Start here for Aikido web app, API and attack surface security.",
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_list_subdomains", method: http.MethodGet, path: "/domains/{domain_id}/subdomains", scope: "domains:read", access: mcp.AccessRead,
		desc:   "List subdomains Aikido discovered for an attack surface domain, with IP addresses, open ports and services, and whether each responds.",
		params: ps(integer("domain_id", "Domain id from aikido_list_domains.")),
		page:   pages(10, 100, 20),
	},
	{
		tool: "aikido_list_domain_data_leaks", method: http.MethodGet, path: "/domains/{domain_id}/dataleaks", scope: "domains:read", access: mcp.AccessRead,
		desc:   "List leaked credentials (data breach leaks) Aikido found for accounts on an attack surface domain, with email, status and last breach date. Leaked passwords are redacted.",
		params: ps(integer("domain_id", "Domain id from aikido_list_domains.")),
		redact: redactFields("password"),
	},
	{
		tool: "aikido_create_domain", method: http.MethodPost, path: "/domains", scope: "domains:write", access: mcp.AccessWrite,
		desc: "Add a domain, web app front end or REST/GraphQL API to Aikido surface monitoring. API kinds need an OpenAPI spec URL, base64 spec, or Zen service id.",
		params: ps(
			str("domain", "Domain name or URL.").req(),
			str("kind", "What to scan.").req().oneOf("front_end", "rest_api", "graphql_api", "attack_surface"),
			str("openapi_spec_url", "URL of the JSON or YAML OpenAPI spec (API kinds)."),
			str("openapi_spec_base64", "Base64-encoded JSON or YAML OpenAPI spec (API kinds)."),
			integer("zen_service_id", "Zen app id that generates the OpenAPI spec (API kinds)."),
		),
	},
	{
		tool: "aikido_start_domain_scan", method: http.MethodPost, path: "/domains/scan", scope: "domains:write", access: mcp.AccessWrite,
		desc:   "Start an Aikido surface monitoring (DAST) scan of a domain now.",
		params: ps(integer("domain_id", "Domain id from aikido_list_domains.").req()),
	},
	{
		tool: "aikido_update_domain_scan_frequency", method: http.MethodPost, path: "/domains/{domain_id}/scanFrequency", scope: "domains:write", access: mcp.AccessWrite,
		desc: "Set how often Aikido scans a domain.",
		params: ps(
			integer("domain_id", "Domain id from aikido_list_domains."),
			str("scan_frequency", "Scan frequency.").req().oneOf("daily", "twice_a_week", "weekly", "monthly", "on_demand"),
		),
	},
	{
		tool: "aikido_add_subdomain", method: http.MethodPost, path: "/domains/{domain_id}/subdomains", scope: "domains:write", access: mcp.AccessWrite,
		desc: "Add a subdomain to an Aikido attack surface domain.",
		params: ps(
			integer("domain_id", "Attack surface domain id."),
			str("subdomain_name", "Subdomain label only, for example api for api.example.com.").req(),
		),
	},
	{
		tool: "aikido_remove_subdomain", method: http.MethodDelete, path: "/domains/{domain_id}/subdomains/{subdomain_id}", scope: "domains:write", access: mcp.AccessWrite,
		desc: "Remove a subdomain from an Aikido attack surface domain. Re-add it with aikido_add_subdomain.",
		params: ps(
			integer("domain_id", "Attack surface domain id."),
			str("subdomain_id", "Subdomain id from aikido_list_subdomains."),
		),
	},
	{
		tool: "aikido_remove_domain", method: http.MethodDelete, path: "/domains/{domain_id}", scope: "domains:write", access: mcp.AccessDestructive,
		desc:   "Remove a domain or API from Aikido surface monitoring with its findings.",
		params: ps(integer("domain_id", "Domain id from aikido_list_domains.")),
	},
}
