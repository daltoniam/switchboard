package cloudflare

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Zones ────────────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_zones",
		Description: "List Cloudflare zones (domains). Start here for DNS management, CDN configuration, and website performance. Zones represent domains registered with Cloudflare.",
		Parameters: map[string]string{
			"name":     "Zone name filter (exact domain match, e.g. example.com)",
			"status":   "Zone status filter (active, pending, initializing, moved, deleted, deactivated, read_only)",
			"page":     "Page number (default 1)",
			"per_page": "Results per page (default 20, max 50)",
		},
	},
	{
		Name:        "cloudflare_get_zone",
		Description: "Get details for a specific Cloudflare zone including status, nameservers, and settings. Use after list_zones.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        "cloudflare_create_zone",
		Description: "Add a new domain (zone) to Cloudflare. Requires account_id.",
		Parameters: map[string]string{
			"name":       "Domain name (e.g. example.com)",
			"account_id": "Account identifier (defaults to configured account_id)",
			"type":       "Zone type: full (default) or partial",
			"jump_start": "Auto-fetch DNS records (true/false, default false)",
		},
		Required: []string{"name"},
	},
	{
		Name:        "cloudflare_edit_zone",
		Description: "Update zone settings like paused status or vanity nameservers. Use after get_zone.",
		Parameters: map[string]string{
			"zone_id": "Zone identifier",
			"paused":  "Pause Cloudflare for this zone (true/false)",
			"plan":    "Plan identifier to change zone plan",
		},
		Required: []string{"zone_id"},
	},
	{
		Name:        "cloudflare_delete_zone",
		Description: "Remove a zone (domain) from Cloudflare. Irreversible.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        "cloudflare_purge_cache",
		Description: "Purge cached content for a Cloudflare zone. Purge everything or specific URLs/tags/hosts. Use for cache invalidation after deployments.",
		Parameters: map[string]string{
			"zone_id":          "Zone identifier",
			"purge_everything": "Purge all cached content (true/false)",
			"files":            "JSON array of URLs to purge (alternative to purge_everything)",
			"tags":             "Comma-separated cache tags to purge",
			"hosts":            "Comma-separated hostnames to purge",
		},
		Required: []string{"zone_id"},
	},

	// ── DNS Records ──────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_dns_records",
		Description: "List DNS records for a Cloudflare zone. View A, AAAA, CNAME, MX, TXT, NS, and other record types. Start here for DNS management and troubleshooting.",
		Parameters: map[string]string{
			"zone_id":  "Zone identifier",
			"type":     "DNS record type filter (A, AAAA, CNAME, MX, TXT, NS, SRV, etc.)",
			"name":     "DNS record name filter (e.g. subdomain.example.com)",
			"content":  "DNS record content filter (e.g. IP address)",
			"page":     "Page number (default 1)",
			"per_page": "Results per page (default 20, max 100)",
		},
		Required: []string{"zone_id"},
	},
	{
		Name:        "cloudflare_get_dns_record",
		Description: "Get details for a specific DNS record. Use after list_dns_records.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "record_id": "DNS record identifier"},
		Required:    []string{"zone_id", "record_id"},
	},
	{
		Name:        "cloudflare_create_dns_record",
		Description: "Create a new DNS record in a Cloudflare zone. Add A, AAAA, CNAME, MX, TXT records and more.",
		Parameters: map[string]string{
			"zone_id":  "Zone identifier",
			"type":     "DNS record type (A, AAAA, CNAME, MX, TXT, NS, SRV, etc.)",
			"name":     "DNS record name (e.g. subdomain or @ for root)",
			"content":  "DNS record content (e.g. IP address, target domain)",
			"ttl":      "TTL in seconds (1 = automatic, default 1)",
			"proxied":  "Whether traffic is proxied through Cloudflare (true/false)",
			"priority": "MX/SRV record priority",
		},
		Required: []string{"zone_id", "type", "name", "content"},
	},
	{
		Name:        "cloudflare_update_dns_record",
		Description: "Update an existing DNS record. Use after list_dns_records to get record_id.",
		Parameters: map[string]string{
			"zone_id":   "Zone identifier",
			"record_id": "DNS record identifier",
			"type":      "DNS record type",
			"name":      "DNS record name",
			"content":   "DNS record content",
			"ttl":       "TTL in seconds (1 = automatic)",
			"proxied":   "Whether traffic is proxied through Cloudflare (true/false)",
			"priority":  "MX/SRV record priority",
		},
		Required: []string{"zone_id", "record_id", "type", "name", "content"},
	},
	{
		Name:        "cloudflare_delete_dns_record",
		Description: "Delete a DNS record from a Cloudflare zone.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "record_id": "DNS record identifier"},
		Required:    []string{"zone_id", "record_id"},
	},

	// ── Workers ──────────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_workers",
		Description: "List Cloudflare Workers scripts. View serverless functions, edge computing workers, and deployed scripts. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        "cloudflare_get_worker",
		Description: "Get metadata for a specific Cloudflare Worker script including bindings, routes, and compatibility settings. Use after list_workers.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "script_name": "Worker script name"},
		Required:    []string{"script_name"},
	},
	{
		Name:        "cloudflare_delete_worker",
		Description: "Delete a Cloudflare Worker script.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "script_name": "Worker script name"},
		Required:    []string{"script_name"},
	},
	{
		Name:        "cloudflare_list_worker_routes",
		Description: "List Worker routes for a zone. Shows URL patterns mapped to Worker scripts.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},

	// ── Pages ────────────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_pages_projects",
		Description: "List Cloudflare Pages projects. View static sites, Jamstack deployments, and frontend projects hosted on Pages. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        "cloudflare_get_pages_project",
		Description: "Get details for a Cloudflare Pages project including build config, domains, and deployment settings. Use after list_pages_projects.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name"},
		Required:    []string{"project_name"},
	},
	{
		Name:        "cloudflare_list_pages_deployments",
		Description: "List deployments for a Cloudflare Pages project. View deploy history, build status, and rollback targets.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name"},
		Required:    []string{"project_name"},
	},
	{
		Name:        "cloudflare_get_pages_deployment",
		Description: "Get details for a specific Pages deployment including build logs and environment. Use after list_pages_deployments.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name", "deployment_id": "Deployment identifier"},
		Required:    []string{"project_name", "deployment_id"},
	},
	{
		Name:        "cloudflare_delete_pages_deployment",
		Description: "Delete a specific Pages deployment.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name", "deployment_id": "Deployment identifier"},
		Required:    []string{"project_name", "deployment_id"},
	},
	{
		Name:        "cloudflare_rollback_pages_deployment",
		Description: "Rollback a Pages project to a previous deployment. Use after list_pages_deployments to find deployment_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name", "deployment_id": "Deployment identifier to rollback to"},
		Required:    []string{"project_name", "deployment_id"},
	},

	// ── R2 ───────────────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_r2_buckets",
		Description: "List R2 object storage buckets. Cloudflare R2 is S3-compatible storage with zero egress fees. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        "cloudflare_create_r2_bucket",
		Description: "Create a new R2 object storage bucket.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "name": "Bucket name"},
		Required:    []string{"name"},
	},
	{
		Name:        "cloudflare_delete_r2_bucket",
		Description: "Delete an R2 object storage bucket. Bucket must be empty.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "name": "Bucket name"},
		Required:    []string{"name"},
	},

	// ── KV ───────────────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_kv_namespaces",
		Description: "List Workers KV namespaces. KV is Cloudflare's global key-value store for edge data. Requires account_id.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
	{
		Name:        "cloudflare_create_kv_namespace",
		Description: "Create a new Workers KV namespace.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "title": "Namespace title"},
		Required:    []string{"title"},
	},
	{
		Name:        "cloudflare_delete_kv_namespace",
		Description: "Delete a Workers KV namespace and all its keys.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "namespace_id": "KV namespace identifier"},
		Required:    []string{"namespace_id"},
	},
	{
		Name:        "cloudflare_list_kv_keys",
		Description: "List keys in a Workers KV namespace. Returns key names with optional metadata.",
		Parameters: map[string]string{
			"account_id":   "Account identifier (defaults to configured account_id)",
			"namespace_id": "KV namespace identifier",
			"prefix":       "Key prefix filter",
			"limit":        "Maximum keys to return (default 1000)",
			"cursor":       "Pagination cursor from previous response",
		},
		Required: []string{"namespace_id"},
	},
	{
		Name:        "cloudflare_get_kv_value",
		Description: "Read a value from Workers KV by key. Returns the stored value as text.",
		Parameters: map[string]string{
			"account_id":   "Account identifier (defaults to configured account_id)",
			"namespace_id": "KV namespace identifier",
			"key_name":     "Key name to read",
		},
		Required: []string{"namespace_id", "key_name"},
	},
	{
		Name:        "cloudflare_put_kv_value",
		Description: "Write a key-value pair to Workers KV.",
		Parameters: map[string]string{
			"account_id":   "Account identifier (defaults to configured account_id)",
			"namespace_id": "KV namespace identifier",
			"key_name":     "Key name",
			"value":        "Value to store",
			"metadata":     "JSON metadata object to attach to the key",
		},
		Required: []string{"namespace_id", "key_name", "value"},
	},
	{
		Name:        "cloudflare_delete_kv_value",
		Description: "Delete a key-value pair from Workers KV.",
		Parameters: map[string]string{
			"account_id":   "Account identifier (defaults to configured account_id)",
			"namespace_id": "KV namespace identifier",
			"key_name":     "Key name to delete",
		},
		Required: []string{"namespace_id", "key_name"},
	},

	// ── D1 ───────────────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_d1_databases",
		Description: "List D1 SQL databases. D1 is Cloudflare's serverless SQLite database for Workers. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        "cloudflare_get_d1_database",
		Description: "Get details for a specific D1 database including size and table count. Use after list_d1_databases.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "database_id": "D1 database identifier"},
		Required:    []string{"database_id"},
	},
	{
		Name:        "cloudflare_create_d1_database",
		Description: "Create a new D1 SQL database.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "name": "Database name"},
		Required:    []string{"name"},
	},
	{
		Name:        "cloudflare_delete_d1_database",
		Description: "Delete a D1 SQL database and all its data. Irreversible.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "database_id": "D1 database identifier"},
		Required:    []string{"database_id"},
	},
	{
		Name:        "cloudflare_query_d1_database",
		Description: "Execute a SQL query against a D1 database. Supports SELECT, INSERT, UPDATE, DELETE and DDL. Use parameterized queries with params array for safety.",
		Parameters: map[string]string{
			"account_id":  "Account identifier (defaults to configured account_id)",
			"database_id": "D1 database identifier",
			"sql":         "SQL query to execute",
			"params":      "JSON array of query parameters for prepared statements",
		},
		Required: []string{"database_id", "sql"},
	},

	// ── Firewall / WAF ──────────────────────────────────────────────
	{
		Name:        "cloudflare_list_waf_rulesets",
		Description: "List WAF (Web Application Firewall) rulesets for a zone. View security rules protecting against attacks, bots, and threats.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        "cloudflare_get_waf_ruleset",
		Description: "Get details for a specific WAF ruleset including individual rules and their actions. Use after list_waf_rulesets.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "ruleset_id": "Ruleset identifier"},
		Required:    []string{"zone_id", "ruleset_id"},
	},

	// ── Load Balancers ──────────────────────────────────────────────
	{
		Name:        "cloudflare_list_load_balancers",
		Description: "List load balancers for a zone. View traffic distribution, health checks, and failover configuration.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        "cloudflare_get_load_balancer",
		Description: "Get details for a specific load balancer including pools, rules, and session affinity. Use after list_load_balancers.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "lb_id": "Load balancer identifier"},
		Required:    []string{"zone_id", "lb_id"},
	},
	{
		Name:        "cloudflare_list_lb_pools",
		Description: "List load balancer pools (origin server groups). View pool health, origins, and traffic steering. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        "cloudflare_get_lb_pool",
		Description: "Get details for a specific load balancer pool including origins and health check results. Use after list_lb_pools.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "pool_id": "Pool identifier"},
		Required:    []string{"pool_id"},
	},
	{
		Name:        "cloudflare_list_lb_monitors",
		Description: "List load balancer health monitors. View health check configurations for origin pools. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},

	// ── Analytics ────────────────────────────────────────────────────
	{
		Name:        "cloudflare_get_zone_analytics",
		Description: "Get zone analytics dashboard data including requests, bandwidth, threats, and page views. Provides traffic overview and performance metrics for a Cloudflare zone.",
		Parameters: map[string]string{
			"zone_id": "Zone identifier",
			"since":   "Start time (ISO 8601 or relative like -1440 for minutes ago, default -1440)",
			"until":   "End time (ISO 8601 or relative, default 0 for now)",
		},
		Required: []string{"zone_id"},
	},

	// ── Accounts ─────────────────────────────────────────────────────
	{
		Name:        "cloudflare_list_accounts",
		Description: "List Cloudflare accounts the API token has access to.",
		Parameters: map[string]string{
			"page":     "Page number (default 1)",
			"per_page": "Results per page (default 20)",
		},
	},
	{
		Name:        "cloudflare_get_account",
		Description: "Get details for a specific Cloudflare account. Use after list_accounts.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        "cloudflare_list_account_members",
		Description: "List members of a Cloudflare account with their roles and permissions.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
}
