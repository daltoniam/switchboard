package cloudflare

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Zones ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_zones"),
		Description: "List Cloudflare zones (domains). Start here for DNS management, CDN configuration, and website performance. Zones represent domains registered with Cloudflare.",
		Parameters: map[string]string{
			"name":     "Zone name filter (exact domain match, e.g. example.com)",
			"status":   "Zone status filter (active, pending, initializing, moved, deleted, deactivated, read_only)",
			"page":     "Page number (default 1)",
			"per_page": "Results per page (default 20, max 50)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_zone"),
		Description: "Get details for a specific Cloudflare zone including status, nameservers, and settings. Use after list_zones.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_zone"),
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
		Name:        mcp.ToolName("cloudflare_edit_zone"),
		Description: "Update zone settings like paused status or vanity nameservers. Use after get_zone.",
		Parameters: map[string]string{
			"zone_id": "Zone identifier",
			"paused":  "Pause Cloudflare for this zone (true/false)",
			"plan":    "Plan identifier to change zone plan",
		},
		Required: []string{"zone_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_zone"),
		Description: "Remove a zone (domain) from Cloudflare. Irreversible.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_purge_cache"),
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
		Name:        mcp.ToolName("cloudflare_list_dns_records"),
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
		Name:        mcp.ToolName("cloudflare_get_dns_record"),
		Description: "Get details for a specific DNS record. Use after list_dns_records.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "record_id": "DNS record identifier"},
		Required:    []string{"zone_id", "record_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_dns_record"),
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
		Name:        mcp.ToolName("cloudflare_update_dns_record"),
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
		Name:        mcp.ToolName("cloudflare_delete_dns_record"),
		Description: "Delete a DNS record from a Cloudflare zone.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "record_id": "DNS record identifier"},
		Required:    []string{"zone_id", "record_id"},
	},

	// ── Workers ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_workers"),
		Description: "List Cloudflare Workers scripts. View serverless functions, edge computing workers, and deployed scripts. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_worker"),
		Description: "Get metadata for a specific Cloudflare Worker script including bindings, routes, and compatibility settings. Use after list_workers.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "script_name": "Worker script name"},
		Required:    []string{"script_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_worker"),
		Description: "Delete a Cloudflare Worker script.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "script_name": "Worker script name"},
		Required:    []string{"script_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_worker_routes"),
		Description: "List Worker routes for a zone. Shows URL patterns mapped to Worker scripts.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},

	// ── Pages ────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_pages_projects"),
		Description: "List Cloudflare Pages projects. View static sites, Jamstack deployments, and frontend projects hosted on Pages. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_pages_project"),
		Description: "Get details for a Cloudflare Pages project including build config, domains, and deployment settings. Use after list_pages_projects.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name"},
		Required:    []string{"project_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_pages_deployments"),
		Description: "List deployments for a Cloudflare Pages project. View deploy history, build status, and rollback targets.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name"},
		Required:    []string{"project_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_pages_deployment"),
		Description: "Get details for a specific Pages deployment including build logs and environment. Use after list_pages_deployments.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name", "deployment_id": "Deployment identifier"},
		Required:    []string{"project_name", "deployment_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_pages_deployment"),
		Description: "Delete a specific Pages deployment.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name", "deployment_id": "Deployment identifier"},
		Required:    []string{"project_name", "deployment_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_rollback_pages_deployment"),
		Description: "Rollback a Pages project to a previous deployment. Use after list_pages_deployments to find deployment_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name", "deployment_id": "Deployment identifier to rollback to"},
		Required:    []string{"project_name", "deployment_id"},
	},

	// ── R2 ───────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_r2_buckets"),
		Description: "List R2 object storage buckets. Cloudflare R2 is S3-compatible storage with zero egress fees. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_r2_bucket"),
		Description: "Create a new R2 object storage bucket.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "name": "Bucket name"},
		Required:    []string{"name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_r2_bucket"),
		Description: "Delete an R2 object storage bucket. Bucket must be empty.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "name": "Bucket name"},
		Required:    []string{"name"},
	},

	// ── KV ───────────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_kv_namespaces"),
		Description: "List Workers KV namespaces. KV is Cloudflare's global key-value store for edge data. Requires account_id.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_kv_namespace"),
		Description: "Create a new Workers KV namespace.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "title": "Namespace title"},
		Required:    []string{"title"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_kv_namespace"),
		Description: "Delete a Workers KV namespace and all its keys.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "namespace_id": "KV namespace identifier"},
		Required:    []string{"namespace_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_kv_keys"),
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
		Name:        mcp.ToolName("cloudflare_get_kv_value"),
		Description: "Read a value from Workers KV by key. Returns the stored value as text.",
		Parameters: map[string]string{
			"account_id":   "Account identifier (defaults to configured account_id)",
			"namespace_id": "KV namespace identifier",
			"key_name":     "Key name to read",
		},
		Required: []string{"namespace_id", "key_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_put_kv_value"),
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
		Name:        mcp.ToolName("cloudflare_delete_kv_value"),
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
		Name:        mcp.ToolName("cloudflare_list_d1_databases"),
		Description: "List D1 SQL databases. D1 is Cloudflare's serverless SQLite database for Workers. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_d1_database"),
		Description: "Get details for a specific D1 database including size and table count. Use after list_d1_databases.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "database_id": "D1 database identifier"},
		Required:    []string{"database_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_d1_database"),
		Description: "Create a new D1 SQL database.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "name": "Database name"},
		Required:    []string{"name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_d1_database"),
		Description: "Delete a D1 SQL database and all its data. Irreversible.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "database_id": "D1 database identifier"},
		Required:    []string{"database_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_query_d1_database"),
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
		Name:        mcp.ToolName("cloudflare_list_waf_rulesets"),
		Description: "List WAF (Web Application Firewall) rulesets for a zone. View security rules protecting against attacks, bots, and threats.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_waf_ruleset"),
		Description: "Get details for a specific WAF ruleset including individual rules and their actions. Use after list_waf_rulesets.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "ruleset_id": "Ruleset identifier"},
		Required:    []string{"zone_id", "ruleset_id"},
	},

	// ── Load Balancers ──────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_load_balancers"),
		Description: "List load balancers for a zone. View traffic distribution, health checks, and failover configuration.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_load_balancer"),
		Description: "Get details for a specific load balancer including pools, rules, and session affinity. Use after list_load_balancers.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "lb_id": "Load balancer identifier"},
		Required:    []string{"zone_id", "lb_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_lb_pools"),
		Description: "List load balancer pools (origin server groups). View pool health, origins, and traffic steering. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_lb_pool"),
		Description: "Get details for a specific load balancer pool including origins and health check results. Use after list_lb_pools.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "pool_id": "Pool identifier"},
		Required:    []string{"pool_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_lb_monitors"),
		Description: "List load balancer health monitors. View health check configurations for origin pools. Requires account_id.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},

	// ── Analytics ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_get_zone_analytics"),
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
		Name:        mcp.ToolName("cloudflare_list_accounts"),
		Description: "List Cloudflare accounts the API token has access to.",
		Parameters: map[string]string{
			"page":     "Page number (default 1)",
			"per_page": "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_account"),
		Description: "Get details for a specific Cloudflare account. Use after list_accounts.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_account_members"),
		Description: "List members of a Cloudflare account with their roles and permissions.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},

	// ── AI Gateway ───────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_ai_gateways"),
		Description: "List Cloudflare AI Gateways for an account. AI Gateway proxies LLM traffic to OpenAI, Anthropic, Workers AI, and other providers with caching, rate limiting, logging, and cost tracking. Start here to discover gateway IDs before pulling logs, token usage, or spend.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_ai_gateway"),
		Description: "Get a Cloudflare AI Gateway's configuration: cache TTL, rate limits, log collection, spend limits, authentication, DLP, and guardrails. Use after list_ai_gateways.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"gateway_id": "AI Gateway identifier",
		},
		Required: []string{"gateway_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_ai_gateway_logs"),
		Description: "List AI Gateway request logs with per-request token usage (tokens_in, tokens_out), cost, latency, model, provider, cache hits, and success/error status. Use this to audit LLM spend, debug failing inferences, or attribute token consumption to a model or provider. Filter by date, provider, model, success, cached, or feedback.",
		Parameters: map[string]string{
			"account_id":         "Account identifier (defaults to configured account_id)",
			"gateway_id":         "AI Gateway identifier",
			"start_date":         "Start of time range (ISO 8601, e.g. 2024-01-01T00:00:00Z)",
			"end_date":           "End of time range (ISO 8601)",
			"provider":           "Filter by LLM provider (e.g. openai, anthropic, workers-ai)",
			"model":              "Filter by model id (e.g. gpt-4o, claude-3-5-sonnet)",
			"success":            "Filter by success (true/false)",
			"cached":             "Filter by cache hit (true/false)",
			"feedback":           "Filter by feedback rating (0 or 1)",
			"search":             "Free-text search across log content",
			"order_by":           "Sort field: created_at, provider, model, model_type, success, or cached",
			"order_by_direction": "Sort direction (asc or desc)",
			"page":               "Page number (default 1)",
			"per_page":           "Results per page (default 20, max 50)",
		},
		Required: []string{"gateway_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_ai_gateway_log"),
		Description: "Get a single AI Gateway log entry with full metadata: tokens_in, tokens_out, cost, model, provider, duration, cached, status. Use after list_ai_gateway_logs to drill into a specific request.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"gateway_id": "AI Gateway identifier",
			"log_id":     "Log entry identifier",
		},
		Required: []string{"gateway_id", "log_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_ai_gateway_log_request"),
		Description: "Get the raw request body (prompt, messages, parameters) sent to the LLM for an AI Gateway log entry. Use after get_ai_gateway_log to inspect the exact prompt that produced a response.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"gateway_id": "AI Gateway identifier",
			"log_id":     "Log entry identifier",
		},
		Required: []string{"gateway_id", "log_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_ai_gateway_log_response"),
		Description: "Get the raw response body (completion, choices, usage) returned by the LLM for an AI Gateway log entry. Use after get_ai_gateway_log to inspect the model's reply or error.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"gateway_id": "AI Gateway identifier",
			"log_id":     "Log entry identifier",
		},
		Required: []string{"gateway_id", "log_id"},
	},

	// ── Workers AI ───────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_ai_models"),
		Description: "List Workers AI models available to run on Cloudflare's edge. Workers AI hosts open-source LLMs, embeddings, image, and speech models. Start here to discover model ids (e.g. @cf/meta/llama-3.1-8b-instruct) before calling run_ai_model.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"search":     "Free-text search filter (e.g. llama, embedding)",
			"task":       "Filter by task type (e.g. Text Generation, Text Embeddings, Speech Recognition)",
			"source":     "Filter by source (1 for first-party, 2 for Hugging Face)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_run_ai_model"),
		Description: "Run inference on a Workers AI model — text generation, embeddings, classification, translation, speech-to-text, image generation. Pass either `prompt` (string) for simple text-in, or `body` (object) for the model's full request schema (messages, input, image, audio, etc.). Use after list_ai_models.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"model_name": "Full model identifier (e.g. @cf/meta/llama-3.1-8b-instruct)",
			"prompt":     "Convenience: text prompt (wrapped as {\"prompt\":...}). Ignored if `body` is set.",
			"body":       "Full inference request body as a JSON object (model-specific schema)",
		},
		Required: []string{"model_name"},
	},

	// ── Vectorize ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_vectorize_indexes"),
		Description: "List Vectorize v2 indexes. Vectorize is Cloudflare's globally distributed vector database for semantic search, RAG, and embeddings. Start here to discover index names before querying.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_vectorize_index"),
		Description: "Get a Vectorize index's configuration: dimensions, distance metric, vector count. Use after list_vectorize_indexes.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "index_name": "Vectorize index name"},
		Required:    []string{"index_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_vectorize_index"),
		Description: "Create a new Vectorize v2 index for storing embeddings. Pick a metric (cosine, euclidean, dot-product) and dimension count (e.g. 768, 1536) that match your embedding model.",
		Parameters: map[string]string{
			"account_id":  "Account identifier (defaults to configured account_id)",
			"name":        "Index name",
			"metric":      "Distance metric: cosine, euclidean, or dot-product",
			"dimensions":  "Vector dimension count (e.g. 768, 1024, 1536)",
			"description": "Optional human description",
		},
		Required: []string{"name", "metric", "dimensions"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_vectorize_index"),
		Description: "Delete a Vectorize index and all its vectors. Irreversible.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "index_name": "Vectorize index name"},
		Required:    []string{"index_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_query_vectorize_index"),
		Description: "Query a Vectorize index by vector — returns the topK nearest neighbors. Use for semantic search, RAG retrieval, recommendation lookup.",
		Parameters: map[string]string{
			"account_id":     "Account identifier (defaults to configured account_id)",
			"index_name":     "Vectorize index name",
			"vector":         "Query vector as JSON array of floats",
			"topK":           "Number of nearest neighbors to return (default 5)",
			"returnValues":   "Whether to return the matched vectors (true/false)",
			"returnMetadata": "Whether to return metadata: 'none', 'indexed', or 'all'",
			"namespace":      "Optional namespace to scope the query",
			"filter":         "Optional metadata filter (JSON object)",
		},
		Required: []string{"index_name", "vector"},
	},

	// ── Queues ───────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_queues"),
		Description: "List Cloudflare Queues. Queues are durable message queues for Workers — producer/consumer messaging, batch processing, async work. Start here to discover queue IDs.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_queue"),
		Description: "Get a Queue's details: producers, consumers, message counts, settings. Use after list_queues.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "queue_id": "Queue identifier"},
		Required:    []string{"queue_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_queue"),
		Description: "Create a new Cloudflare Queue.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"queue_name": "Queue name",
			"settings":   "Optional settings map (delivery_delay, message_retention_period, etc.)",
		},
		Required: []string{"queue_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_queue"),
		Description: "Delete a Cloudflare Queue and all pending messages. Irreversible.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "queue_id": "Queue identifier"},
		Required:    []string{"queue_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_send_queue_messages"),
		Description: "Publish messages to a Cloudflare Queue. Messages is a JSON array of objects with at least a `body` field.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"queue_id":   "Queue identifier",
			"messages":   "JSON array of message objects (each with `body` and optional `content_type`, `delay_seconds`)",
		},
		Required: []string{"queue_id", "messages"},
	},

	// ── Hyperdrive ───────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_hyperdrive_configs"),
		Description: "List Hyperdrive configs. Hyperdrive accelerates database connections from Workers by pooling and caching at the edge. Start here to discover config IDs.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_hyperdrive_config"),
		Description: "Get a Hyperdrive config's details: origin connection, caching, mTLS. Use after list_hyperdrive_configs.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "hyperdrive_id": "Hyperdrive config identifier"},
		Required:    []string{"hyperdrive_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_hyperdrive_config"),
		Description: "Create a Hyperdrive config that proxies and caches a database connection (Postgres, MySQL) for Workers.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"name":       "Config name",
			"origin":     "Origin connection JSON: {scheme, host, port, database, user, password}",
			"caching":    "Optional caching JSON: {disabled, max_age, stale_while_revalidate}",
			"mtls":       "Optional mTLS JSON",
		},
		Required: []string{"name", "origin"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_hyperdrive_config"),
		Description: "Delete a Hyperdrive config. Workers using this config will fail until rebound.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "hyperdrive_id": "Hyperdrive config identifier"},
		Required:    []string{"hyperdrive_id"},
	},

	// ── Workers extras ───────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_worker_secrets"),
		Description: "List secret names bound to a Worker script. Values are never returned by the API. Use to audit which secrets a Worker has access to.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "script_name": "Worker script name"},
		Required:    []string{"script_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_worker_deployments"),
		Description: "List recent deployments for a Worker script with versions, authors, timestamps. Use to inspect deploy history or find a version to roll back to.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "script_name": "Worker script name"},
		Required:    []string{"script_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_worker_subdomain"),
		Description: "Get the workers.dev subdomain enabled for this account (used as the default Worker URL).",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_worker_tails"),
		Description: "List active Worker tails (live log streams) for a script. Use to find existing tails before opening a new live-log session.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "script_name": "Worker script name"},
		Required:    []string{"script_name"},
	},

	// ── Pages create/list domains ───────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_create_pages_project"),
		Description: "Create a Cloudflare Pages project (static site / Jamstack app).",
		Parameters: map[string]string{
			"account_id":         "Account identifier (defaults to configured account_id)",
			"name":               "Project name",
			"production_branch":  "Production branch (e.g. main)",
			"build_config":       "Optional build config JSON: {build_command, destination_dir, root_dir, web_analytics_tag}",
			"source":             "Optional source JSON: {type, config: {owner, repo_name, production_branch, ...}}",
			"deployment_configs": "Optional deployment_configs JSON: {production, preview}",
		},
		Required: []string{"name", "production_branch"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_pages_deployment"),
		Description: "Trigger a new Pages deployment, optionally targeting a specific branch.",
		Parameters: map[string]string{
			"account_id":   "Account identifier (defaults to configured account_id)",
			"project_name": "Pages project name",
			"branch":       "Optional branch to deploy (defaults to production)",
		},
		Required: []string{"project_name"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_pages_domains"),
		Description: "List custom domains attached to a Pages project (DNS + cert state).",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "project_name": "Pages project name"},
		Required:    []string{"project_name"},
	},

	// ── KV bulk ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_bulk_delete_kv_values"),
		Description: "Delete multiple keys from a Workers KV namespace in one call (up to 10000 keys).",
		Parameters: map[string]string{
			"account_id":   "Account identifier (defaults to configured account_id)",
			"namespace_id": "KV namespace identifier",
			"keys":         "JSON array of key names to delete",
		},
		Required: []string{"namespace_id", "keys"},
	},

	// ── Stream ───────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_stream_videos"),
		Description: "List videos hosted on Cloudflare Stream. Start here to discover video IDs for playback URLs, analytics, or deletion.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"after":      "Show videos uploaded after this ISO 8601 timestamp",
			"before":     "Show videos uploaded before this ISO 8601 timestamp",
			"creator":    "Filter by creator id",
			"status":     "Filter by status (queued, inprogress, ready, error)",
			"search":     "Free-text search across video names",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_stream_video"),
		Description: "Get a Stream video's metadata: playback URLs (HLS/DASH), thumbnail, duration, status. Use after list_stream_videos.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "identifier": "Stream video identifier"},
		Required:    []string{"identifier"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_stream_video"),
		Description: "Delete a Stream video. Irreversible.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "identifier": "Stream video identifier"},
		Required:    []string{"identifier"},
	},

	// ── Images ───────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_images"),
		Description: "List images hosted on Cloudflare Images. Start here to discover image IDs for delivery URLs or deletion.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 50)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_image"),
		Description: "Get a Cloudflare Images record: variants, metadata, upload timestamp.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "image_id": "Image identifier"},
		Required:    []string{"image_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_image"),
		Description: "Delete an image from Cloudflare Images. Irreversible.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "image_id": "Image identifier"},
		Required:    []string{"image_id"},
	},

	// ── Zero Trust Access ────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_access_apps"),
		Description: "List Cloudflare Zero Trust Access applications (apps protected by Cloudflare Access). Start here to discover app IDs before listing policies or auditing access.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 25)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_access_app_policies"),
		Description: "List Access policies bound to a specific application. Shows who can access the app (groups, emails, IPs, IdPs, mTLS). Use after list_access_apps.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "app_id": "Access application identifier"},
		Required:    []string{"app_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_access_identity_providers"),
		Description: "List configured Access identity providers (Google, Okta, Azure AD, SAML, OIDC, GitHub, etc.).",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},

	// ── Cloudflared Tunnels ─────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_tunnels"),
		Description: "List Cloudflared tunnels for an account. Tunnels expose private origins to Cloudflare without inbound ports. Start here to discover tunnel IDs and health.",
		Parameters: map[string]string{
			"account_id":      "Account identifier (defaults to configured account_id)",
			"name":            "Filter by tunnel name",
			"status":          "Filter by status: healthy, degraded, down, inactive",
			"include_deleted": "Include deleted tunnels (true/false)",
			"page":            "Page number (default 1)",
			"per_page":        "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_tunnel"),
		Description: "Get a Cloudflared tunnel's details: name, status, connections, created/deleted timestamps. Use after list_tunnels.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "tunnel_id": "Tunnel identifier"},
		Required:    []string{"tunnel_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_tunnel"),
		Description: "Delete a Cloudflared tunnel. Origins behind it become unreachable.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "tunnel_id": "Tunnel identifier"},
		Required:    []string{"tunnel_id"},
	},

	// ── Email Routing ────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_email_routing_rules"),
		Description: "List Email Routing rules for a zone (which incoming addresses forward where). Start here for email routing config audits.",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_email_routing_addresses"),
		Description: "List verified destination email addresses for Email Routing (account-scoped).",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"verified":   "Filter by verified status (true/false)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_email_routing_settings"),
		Description: "Get Email Routing settings for a zone (enabled state, MX/SPF records, skip_wizard).",
		Parameters:  map[string]string{"zone_id": "Zone identifier"},
		Required:    []string{"zone_id"},
	},

	// ── Logpush ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_logpush_jobs"),
		Description: "List Logpush jobs for an account (HTTP requests, firewall events, Workers traces, etc. exported to S3/GCS/Azure/Sumo/Datadog/Splunk/New Relic/R2). Start here to inspect log-export pipelines.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_logpush_job"),
		Description: "Get a Logpush job's config: dataset, destination, frequency, filters, last error. Use after list_logpush_jobs.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "job_id": "Logpush job ID (integer)"},
		Required:    []string{"job_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_create_logpush_job"),
		Description: "Create a Logpush job to export Cloudflare logs to an external sink (S3, GCS, Azure, R2, Splunk, Datadog, New Relic, Sumo Logic, HTTP).",
		Parameters: map[string]string{
			"account_id":       "Account identifier (defaults to configured account_id)",
			"dataset":          "Log dataset (e.g. http_requests, firewall_events, workers_trace_events)",
			"destination_conf": "Destination connection string (e.g. s3://bucket/path?region=...)",
			"name":             "Optional job name",
			"frequency":        "Optional frequency: high or low",
			"logpull_options":  "Optional logpull options string (fields, timestamps, etc.)",
			"output_options":   "Optional output options map (field_names, timestamp_format, batch sizing)",
			"filter":           "Optional log filter expression",
			"enabled":          "Whether the job is enabled (true/false)",
		},
		Required: []string{"dataset", "destination_conf"},
	},

	// ── Page Rules ───────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_page_rules"),
		Description: "List Page Rules for a zone (URL-pattern rules for cache, redirects, headers, security). Start here for legacy page-rule audits before migrating to Rulesets.",
		Parameters: map[string]string{
			"zone_id":   "Zone identifier",
			"status":    "Filter by status (active, disabled)",
			"order":     "Sort field (priority, status)",
			"direction": "Sort direction (asc, desc)",
		},
		Required: []string{"zone_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_page_rule"),
		Description: "Get a Page Rule's targets and actions. Use after list_page_rules.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "pagerule_id": "Page Rule identifier"},
		Required:    []string{"zone_id", "pagerule_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_page_rule"),
		Description: "Delete a Page Rule.",
		Parameters:  map[string]string{"zone_id": "Zone identifier", "pagerule_id": "Page Rule identifier"},
		Required:    []string{"zone_id", "pagerule_id"},
	},

	// ── Notifications (Alerting v3) ─────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_notification_policies"),
		Description: "List notification (alerting) policies — what conditions trigger emails, PagerDuty, webhooks for an account.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},
	{
		Name:        mcp.ToolName("cloudflare_list_notification_webhooks"),
		Description: "List configured webhook destinations for Cloudflare notifications.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)"},
	},

	// ── Account API Tokens ──────────────────────────────────────────
	{
		Name:        mcp.ToolName("cloudflare_list_api_tokens"),
		Description: "List Cloudflare API tokens issued under an account. Start here to audit which tokens exist, their scopes, and their last-used timestamp.",
		Parameters: map[string]string{
			"account_id": "Account identifier (defaults to configured account_id)",
			"page":       "Page number (default 1)",
			"per_page":   "Results per page (default 20)",
		},
	},
	{
		Name:        mcp.ToolName("cloudflare_get_api_token"),
		Description: "Get a Cloudflare API token's policy details: permissions, IP/time restrictions, expiry, last_used_on. Use after list_api_tokens.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "token_id": "API token identifier"},
		Required:    []string{"token_id"},
	},
	{
		Name:        mcp.ToolName("cloudflare_delete_api_token"),
		Description: "Revoke (delete) a Cloudflare API token. Any client using it will start receiving 401s.",
		Parameters:  map[string]string{"account_id": "Account identifier (defaults to configured account_id)", "token_id": "API token identifier"},
		Required:    []string{"token_id"},
	},
}
