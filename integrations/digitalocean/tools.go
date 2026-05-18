package digitalocean

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Account ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_get_account"), Description: "Get current account information including email, droplet limit, and status. Start here to verify access.",
		Parameters: []mcp.Parameter{},
	},

	// ── Droplets ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_droplets"), Description: "List all droplets in the account",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page (max 200)"}, {Name: mcp.ParamName("tag_name"), Description: "Filter by tag"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_droplet"), Description: "Get details of a specific droplet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("droplet_id"), Description: "Droplet ID (integer)", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_create_droplet"), Description: "Create a new droplet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Droplet name", Required: true}, {Name: mcp.ParamName("region"), Description: "Region slug (e.g. nyc3)", Required: true}, {Name: mcp.ParamName("size"), Description: "Size slug (e.g. s-1vcpu-1gb)", Required: true}, {Name: mcp.ParamName("image"), Description: "Image slug or ID (e.g. ubuntu-24-04-x64)", Required: true}, {Name: mcp.ParamName("ssh_keys"), Description: "Comma-separated SSH key IDs or fingerprints"}, {Name: mcp.ParamName("tags"), Description: "Comma-separated tags"}, {Name: mcp.ParamName("vpc_uuid"), Description: "VPC UUID"}},
	},
	{
		Name: mcp.ToolName("digitalocean_delete_droplet"), Description: "Delete a droplet by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("droplet_id"), Description: "Droplet ID (integer)", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_reboot_droplet"), Description: "Reboot a droplet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("droplet_id"), Description: "Droplet ID (integer)", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_poweroff_droplet"), Description: "Power off a droplet (hard shutdown)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("droplet_id"), Description: "Droplet ID (integer)", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_poweron_droplet"), Description: "Power on a droplet",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("droplet_id"), Description: "Droplet ID (integer)", Required: true}},
	},

	// ── Kubernetes ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_kubernetes_clusters"), Description: "List all Kubernetes clusters",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_kubernetes_cluster"), Description: "Get details of a specific Kubernetes cluster",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cluster_id"), Description: "Cluster UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_kubernetes_node_pools"), Description: "List node pools for a Kubernetes cluster",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cluster_id"), Description: "Cluster UUID", Required: true}},
	},

	// ── Databases ───────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_databases"), Description: "List all managed database clusters",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_database"), Description: "Get details of a managed database cluster",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database cluster UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_database_dbs"), Description: "List databases within a managed database cluster",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database cluster UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_database_users"), Description: "List users for a managed database cluster",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database cluster UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_database_pools"), Description: "List connection pools for a managed database cluster",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database cluster UUID", Required: true}},
	},

	// ── Networking ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_domains"), Description: "List all domains",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_domain"), Description: "Get details of a domain",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("domain_name"), Description: "Domain name (e.g. example.com)", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_domain_records"), Description: "List DNS records for a domain",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("domain_name"), Description: "Domain name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_load_balancers"), Description: "List all load balancers",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_load_balancer"), Description: "Get details of a load balancer",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("lb_id"), Description: "Load balancer UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_firewalls"), Description: "List all cloud firewalls",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_firewall"), Description: "Get details of a cloud firewall",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("firewall_id"), Description: "Firewall UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_vpcs"), Description: "List all VPCs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_vpc"), Description: "Get details of a VPC",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("vpc_id"), Description: "VPC UUID", Required: true}},
	},

	// ── Volumes ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_volumes"), Description: "List all block storage volumes",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}, {Name: mcp.ParamName("region"), Description: "Filter by region slug"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_volume"), Description: "Get details of a block storage volume",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("volume_id"), Description: "Volume UUID", Required: true}},
	},

	// ── Apps ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_apps"), Description: "List all App Platform apps",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app"), Description: "Get details of an App Platform app including its spec and active deployment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_delete_app"), Description: "Delete an App Platform app",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_restart_app"), Description: "Restart an App Platform app, optionally targeting specific components",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}, {Name: mcp.ParamName("components"), Description: "Comma-separated component names to restart (omit to restart all)"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_app_deployments"), Description: "List deployments for an App Platform app",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app_deployment"), Description: "Get details of a specific App Platform deployment",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}, {Name: mcp.ParamName("deployment_id"), Description: "Deployment UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_create_app_deployment"), Description: "Trigger a new deployment for an App Platform app",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}, {Name: mcp.ParamName("force_build"), Description: "Force rebuild even if no source changes (true/false)"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app_logs"), Description: "Get logs for an App Platform app. Use log_type BUILD for build logs, DEPLOY for deploy logs, or RUN for runtime logs",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}, {Name: mcp.ParamName("deployment_id"), Description: "Deployment UUID (omit for active deployment)"}, {Name: mcp.ParamName("component"), Description: "Component name (omit for all components)"}, {Name: mcp.ParamName("log_type"), Description: "Log type: BUILD, DEPLOY, RUN, RUN_RESTARTED, or AUTOSCALE_EVENT", Required: true}, {Name: mcp.ParamName("tail_lines"), Description: "Number of log lines to return (default 100)"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app_health"), Description: "Get health status of all components in an App Platform app",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_app_alerts"), Description: "List alerts configured for an App Platform app",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "App UUID", Required: true}},
	},

	// ── Extras ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_regions"), Description: "List all available regions",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("digitalocean_list_sizes"), Description: "List all available droplet sizes",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("digitalocean_list_images"), Description: "List available images (OS distributions and snapshots)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("type"), Description: "Filter by type: distribution, application, or user"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_ssh_keys"), Description: "List all SSH keys in the account",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_snapshots"), Description: "List all snapshots (droplet and volume)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("resource_type"), Description: "Filter: droplet or volume"}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_projects"), Description: "List all projects",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_project"), Description: "Get details of a project",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("project_id"), Description: "Project UUID", Required: true}},
	},
	{
		Name: mcp.ToolName("digitalocean_get_balance"), Description: "Get current account balance",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: mcp.ToolName("digitalocean_list_invoices"), Description: "List billing invoices",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_cdn_endpoints"), Description: "List all CDN endpoints",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_certificates"), Description: "List all SSL/TLS certificates",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_registries"), Description: "List container registry repositories",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("registry_name"), Description: "Registry name", Required: true}, {Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
	{
		Name: mcp.ToolName("digitalocean_list_tags"), Description: "List all tags",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page"), Description: "Page number"}, {Name: mcp.ParamName("per_page"), Description: "Results per page"}},
	},
}
