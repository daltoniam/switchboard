package cloudflare

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Zones ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_zones"),
		Description: "List Cloudflare zones (domains). Start here for DNS management, CDN configuration, and website performance. Zones represent domains registered with Cloudflare.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Zone name filter (exact domain match, e.g. example.com)"}, {Name: mcp.ParamName("status"), Description: "Zone status filter (active, pending, initializing, moved, deleted, deactivated, read_only)"}, {Name: mcp.ParamName("page"), Description: "Page number (default 1)"}, {Name: mcp.ParamName("per_page"), Description: "Results per page (default 20, max 50)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_zone"),
		Description: "Get details for a specific Cloudflare zone including status, nameservers, and settings. Use after list_zones.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_zone"),
		Description: "Add a new domain (zone) to Cloudflare. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Domain name (e.g. example.com)", Required: true}, {Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("type"), Description: "Zone type: full (default) or partial"}, {Name: mcp.ParamName("jump_start"), Description: "Auto-fetch DNS records (true/false, default false)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_edit_zone"),
		Description: "Update zone settings like paused status or vanity nameservers. Use after get_zone.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("paused"), Description: "Pause Cloudflare for this zone (true/false)"}, {Name: mcp.ParamName("plan"), Description: "Plan identifier to change zone plan"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_zone"),
		Description: "Remove a zone (domain) from Cloudflare. Irreversible.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_purge_cache"),
		Description: "Purge cached content for a Cloudflare zone. Purge everything or specific URLs/tags/hosts. Use for cache invalidation after deployments.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("purge_everything"), Description: "Purge all cached content (true/false)"}, {Name: mcp.ParamName("files"), Description: "JSON array of URLs to purge (alternative to purge_everything)"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated cache tags to purge"}, {Name: mcp.ParamName(

		// ── DNS Records ──────────────────────────────────────────────────
		"hosts"), Description: "Comma-separated hostnames to purge"}},
	},

	{
		Name:        mcp.ToolName("cloudflare_list_dns_records"),
		Description: "List DNS records for a Cloudflare zone. View A, AAAA, CNAME, MX, TXT, NS, and other record types. Start here for DNS management and troubleshooting.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("type"), Description: "DNS record type filter (A, AAAA, CNAME, MX, TXT, NS, SRV, etc.)"}, {Name: mcp.ParamName("name"), Description: "DNS record name filter (e.g. subdomain.example.com)"}, {Name: mcp.ParamName("content"), Description: "DNS record content filter (e.g. IP address)"}, {Name: mcp.ParamName("page"), Description: "Page number (default 1)"}, {Name: mcp.ParamName("per_page"), Description: "Results per page (default 20, max 100)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_dns_record"),
		Description: "Get details for a specific DNS record. Use after list_dns_records.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("record_id"), Description: "DNS record identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_dns_record"),
		Description: "Create a new DNS record in a Cloudflare zone. Add A, AAAA, CNAME, MX, TXT records and more.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("type"), Description: "DNS record type (A, AAAA, CNAME, MX, TXT, NS, SRV, etc.)", Required: true}, {Name: mcp.ParamName("name"), Description: "DNS record name (e.g. subdomain or @ for root)", Required: true}, {Name: mcp.ParamName("content"), Description: "DNS record content (e.g. IP address, target domain)", Required: true}, {Name: mcp.ParamName("ttl"), Description: "TTL in seconds (1 = automatic, default 1)"}, {Name: mcp.ParamName("proxied"), Description: "Whether traffic is proxied through Cloudflare (true/false)"}, {Name: mcp.ParamName("priority"), Description: "MX/SRV record priority"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_update_dns_record"),
		Description: "Update an existing DNS record. Use after list_dns_records to get record_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("record_id"), Description: "DNS record identifier", Required: true}, {Name: mcp.ParamName("type"), Description: "DNS record type", Required: true}, {Name: mcp.ParamName("name"), Description: "DNS record name", Required: true}, {Name: mcp.ParamName("content"), Description: "DNS record content", Required: true}, {Name: mcp.ParamName("ttl"), Description: "TTL in seconds (1 = automatic)"}, {Name: mcp.ParamName("proxied"), Description: "Whether traffic is proxied through Cloudflare (true/false)"}, {Name: mcp.ParamName("priority"), Description: "MX/SRV record priority"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_dns_record"),
		Description: "Delete a DNS record from a Cloudflare zone.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("record_id"), Description: "DNS record identifier",

		// ── Workers ──────────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name:        mcp.ToolName("cloudflare_list_workers"),
		Description: "List Cloudflare Workers scripts. View serverless functions, edge computing workers, and deployed scripts. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_worker"),
		Description: "Get metadata for a specific Cloudflare Worker script including bindings, routes, and compatibility settings. Use after list_workers.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("script_name"), Description: "Worker script name", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_worker"),
		Description: "Delete a Cloudflare Worker script.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("script_name"), Description: "Worker script name", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_worker_routes"),
		Description: "List Worker routes for a zone. Shows URL patterns mapped to Worker scripts.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}},
	},

	// ── Pages ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_pages_projects"),
		Description: "List Cloudflare Pages projects. View static sites, Jamstack deployments, and frontend projects hosted on Pages. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_pages_project"),
		Description: "Get details for a Cloudflare Pages project including build config, domains, and deployment settings. Use after list_pages_projects.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("project_name"), Description: "Pages project name", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_pages_deployments"),
		Description: "List deployments for a Cloudflare Pages project. View deploy history, build status, and rollback targets.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("project_name"), Description: "Pages project name", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_pages_deployment"),
		Description: "Get details for a specific Pages deployment including build logs and environment. Use after list_pages_deployments.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("project_name"), Description: "Pages project name", Required: true}, {Name: mcp.ParamName("deployment_id"), Description: "Deployment identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_pages_deployment"),
		Description: "Delete a specific Pages deployment.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("project_name"), Description: "Pages project name", Required: true}, {Name: mcp.ParamName("deployment_id"), Description: "Deployment identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_rollback_pages_deployment"),
		Description: "Rollback a Pages project to a previous deployment. Use after list_pages_deployments to find deployment_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("project_name"), Description: "Pages project name", Required: true}, {Name: mcp.ParamName("deployment_id"), Description:

		// ── R2 ───────────────────────────────────────────────────────────
		"Deployment identifier to rollback to", Required: true}},
	},

	{
		Name:        mcp.ToolName("cloudflare_list_r2_buckets"),
		Description: "List R2 object storage buckets. Cloudflare R2 is S3-compatible storage with zero egress fees. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_r2_bucket"),
		Description: "Create a new R2 object storage bucket.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("name"), Description: "Bucket name", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_r2_bucket"),
		Description: "Delete an R2 object storage bucket. Bucket must be empty.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("name"), Description:

		// ── KV ───────────────────────────────────────────────────────────
		"Bucket name", Required: true}},
	},

	{
		Name:        mcp.ToolName("cloudflare_list_kv_namespaces"),
		Description: "List Workers KV namespaces. KV is Cloudflare's global key-value store for edge data. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("page"), Description: "Page number (default 1)"}, {Name: mcp.ParamName("per_page"), Description: "Results per page (default 20)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_kv_namespace"),
		Description: "Create a new Workers KV namespace.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("title"), Description: "Namespace title", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_kv_namespace"),
		Description: "Delete a Workers KV namespace and all its keys.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("namespace_id"), Description: "KV namespace identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_kv_keys"),
		Description: "List keys in a Workers KV namespace. Returns key names with optional metadata.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("namespace_id"), Description: "KV namespace identifier", Required: true}, {Name: mcp.ParamName("prefix"), Description: "Key prefix filter"}, {Name: mcp.ParamName("limit"), Description: "Maximum keys to return (default 1000)"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_kv_value"),
		Description: "Read a value from Workers KV by key. Returns the stored value as text.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("namespace_id"), Description: "KV namespace identifier", Required: true}, {Name: mcp.ParamName("key_name"), Description: "Key name to read", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_put_kv_value"),
		Description: "Write a key-value pair to Workers KV.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("namespace_id"), Description: "KV namespace identifier", Required: true}, {Name: mcp.ParamName("key_name"), Description: "Key name", Required: true}, {Name: mcp.ParamName("value"), Description: "Value to store", Required: true}, {Name: mcp.ParamName("metadata"), Description: "JSON metadata object to attach to the key"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_kv_value"),
		Description: "Delete a key-value pair from Workers KV.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("namespace_id"), Description: "KV namespace identifier", Required: true}, {Name: mcp.ParamName("key_name"), Description:

		// ── D1 ───────────────────────────────────────────────────────────
		"Key name to delete", Required: true}},
	},

	{
		Name:        mcp.ToolName("cloudflare_list_d1_databases"),
		Description: "List D1 SQL databases. D1 is Cloudflare's serverless SQLite database for Workers. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_d1_database"),
		Description: "Get details for a specific D1 database including size and table count. Use after list_d1_databases.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("database_id"), Description: "D1 database identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_d1_database"),
		Description: "Create a new D1 SQL database.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("name"), Description: "Database name", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_d1_database"),
		Description: "Delete a D1 SQL database and all its data. Irreversible.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("database_id"), Description: "D1 database identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_query_d1_database"),
		Description: "Execute a SQL query against a D1 database. Supports SELECT, INSERT, UPDATE, DELETE and DDL. Use parameterized queries with params array for safety.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("database_id"), Description: "D1 database identifier", Required: true}, {Name: mcp.ParamName("sql"), Description: "SQL query to execute", Required: true}, {Name: mcp.ParamName("params"),

		// ── Firewall / WAF ──────────────────────────────────────────────
		Description: "JSON array of query parameters for prepared statements"}},
	},

	{
		Name:        mcp.ToolName("cloudflare_list_waf_rulesets"),
		Description: "List WAF (Web Application Firewall) rulesets for a zone. View security rules protecting against attacks, bots, and threats.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_waf_ruleset"),
		Description: "Get details for a specific WAF ruleset including individual rules and their actions. Use after list_waf_rulesets.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("ruleset_id"), Description: "Ruleset identifier",

		// ── Load Balancers ──────────────────────────────────────────────
		Required: true}},
	},

	{
		Name:        mcp.ToolName("cloudflare_list_load_balancers"),
		Description: "List load balancers for a zone. View traffic distribution, health checks, and failover configuration.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_load_balancer"),
		Description: "Get details for a specific load balancer including pools, rules, and session affinity. Use after list_load_balancers.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("lb_id"), Description: "Load balancer identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_lb_pools"),
		Description: "List load balancer pools (origin server groups). View pool health, origins, and traffic steering. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_lb_pool"),
		Description: "Get details for a specific load balancer pool including origins and health check results. Use after list_lb_pools.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("pool_id"), Description: "Pool identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_lb_monitors"),
		Description: "List load balancer health monitors. View health check configurations for origin pools. Requires account_id.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}},
	},

	// ── Analytics ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_get_zone_analytics"),
		Description: "Get zone analytics dashboard data including requests, bandwidth, threats, and page views. Provides traffic overview and performance metrics for a Cloudflare zone.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("zone_id"), Description: "Zone identifier", Required: true}, {Name: mcp.ParamName("since"), Description: "Start time (ISO 8601 or relative like -1440 for minutes ago, default -1440)"}, {Name: mcp.ParamName("until"), Description: "End time (ISO 8601 or relative, default 0 for now)"}},
	},

	// ── Accounts ─────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_accounts"),
		Description: "List Cloudflare accounts the API token has access to.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number (default 1)"}, {Name: mcp.ParamName("per_page"), Description: "Results per page (default 20)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_account"),
		Description: "Get details for a specific Cloudflare account. Use after list_accounts.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_account_members"),
		Description: "List members of a Cloudflare account with their roles and permissions.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("account_id"), Description: "Account identifier (defaults to configured account_id)"}, {Name: mcp.ParamName("page"), Description: "Page number (default 1)"}, {Name: mcp.ParamName("per_page"), Description: "Results per page (default 20)"}},
	},
}
