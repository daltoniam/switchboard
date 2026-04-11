package digitalocean

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Account ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_get_account"), Description: "Get current account information including email, droplet limit, and status. Start here to verify access.",
		Parameters: map[string]string{},
	},

	// ── Droplets ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_droplets"), Description: "List all droplets in the account",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page (max 200)", "tag_name": "Filter by tag"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_droplet"), Description: "Get details of a specific droplet",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_create_droplet"), Description: "Create a new droplet",
		Parameters: map[string]string{"name": "Droplet name", "region": "Region slug (e.g. nyc3)", "size": "Size slug (e.g. s-1vcpu-1gb)", "image": "Image slug or ID (e.g. ubuntu-24-04-x64)", "ssh_keys": "Comma-separated SSH key IDs or fingerprints", "tags": "Comma-separated tags", "vpc_uuid": "VPC UUID"},
		Required:   []string{"name", "region", "size", "image"},
	},
	{
		Name: mcp.ToolName("digitalocean_delete_droplet"), Description: "Delete a droplet by ID",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_reboot_droplet"), Description: "Reboot a droplet",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_poweroff_droplet"), Description: "Power off a droplet (hard shutdown)",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_poweron_droplet"), Description: "Power on a droplet",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},

	// ── Kubernetes ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_kubernetes_clusters"), Description: "List all Kubernetes clusters",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_kubernetes_cluster"), Description: "Get details of a specific Kubernetes cluster",
		Parameters: map[string]string{"cluster_id": "Cluster UUID"},
		Required:   []string{"cluster_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_kubernetes_node_pools"), Description: "List node pools for a Kubernetes cluster",
		Parameters: map[string]string{"cluster_id": "Cluster UUID"},
		Required:   []string{"cluster_id"},
	},

	// ── Databases ───────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_databases"), Description: "List all managed database clusters",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_database"), Description: "Get details of a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_database_dbs"), Description: "List databases within a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_database_users"), Description: "List users for a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_database_pools"), Description: "List connection pools for a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},

	// ── Networking ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_domains"), Description: "List all domains",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_domain"), Description: "Get details of a domain",
		Parameters: map[string]string{"domain_name": "Domain name (e.g. example.com)"},
		Required:   []string{"domain_name"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_domain_records"), Description: "List DNS records for a domain",
		Parameters: map[string]string{"domain_name": "Domain name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"domain_name"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_load_balancers"), Description: "List all load balancers",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_load_balancer"), Description: "Get details of a load balancer",
		Parameters: map[string]string{"lb_id": "Load balancer UUID"},
		Required:   []string{"lb_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_firewalls"), Description: "List all cloud firewalls",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_firewall"), Description: "Get details of a cloud firewall",
		Parameters: map[string]string{"firewall_id": "Firewall UUID"},
		Required:   []string{"firewall_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_vpcs"), Description: "List all VPCs",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_vpc"), Description: "Get details of a VPC",
		Parameters: map[string]string{"vpc_id": "VPC UUID"},
		Required:   []string{"vpc_id"},
	},

	// ── Volumes ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_volumes"), Description: "List all block storage volumes",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page", "region": "Filter by region slug"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_volume"), Description: "Get details of a block storage volume",
		Parameters: map[string]string{"volume_id": "Volume UUID"},
		Required:   []string{"volume_id"},
	},

	// ── Apps ────────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_apps"), Description: "List all App Platform apps",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app"), Description: "Get details of an App Platform app including its spec and active deployment",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_delete_app"), Description: "Delete an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_restart_app"), Description: "Restart an App Platform app, optionally targeting specific components",
		Parameters: map[string]string{"app_id": "App UUID", "components": "Comma-separated component names to restart (omit to restart all)"},
		Required:   []string{"app_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_app_deployments"), Description: "List deployments for an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"app_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app_deployment"), Description: "Get details of a specific App Platform deployment",
		Parameters: map[string]string{"app_id": "App UUID", "deployment_id": "Deployment UUID"},
		Required:   []string{"app_id", "deployment_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_create_app_deployment"), Description: "Trigger a new deployment for an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID", "force_build": "Force rebuild even if no source changes (true/false)"},
		Required:   []string{"app_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app_logs"), Description: "Get logs for an App Platform app. Use log_type BUILD for build logs, DEPLOY for deploy logs, or RUN for runtime logs",
		Parameters: map[string]string{"app_id": "App UUID", "deployment_id": "Deployment UUID (omit for active deployment)", "component": "Component name (omit for all components)", "log_type": "Log type: BUILD, DEPLOY, RUN, RUN_RESTARTED, or AUTOSCALE_EVENT", "tail_lines": "Number of log lines to return (default 100)"},
		Required:   []string{"app_id", "log_type"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_app_health"), Description: "Get health status of all components in an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_app_alerts"), Description: "List alerts configured for an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},

	// ── Extras ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("digitalocean_list_regions"), Description: "List all available regions",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("digitalocean_list_sizes"), Description: "List all available droplet sizes",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("digitalocean_list_images"), Description: "List available images (OS distributions and snapshots)",
		Parameters: map[string]string{"type": "Filter by type: distribution, application, or user", "page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_ssh_keys"), Description: "List all SSH keys in the account",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_snapshots"), Description: "List all snapshots (droplet and volume)",
		Parameters: map[string]string{"resource_type": "Filter: droplet or volume", "page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_projects"), Description: "List all projects",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_project"), Description: "Get details of a project",
		Parameters: map[string]string{"project_id": "Project UUID"},
		Required:   []string{"project_id"},
	},
	{
		Name: mcp.ToolName("digitalocean_get_balance"), Description: "Get current account balance",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("digitalocean_list_invoices"), Description: "List billing invoices",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_cdn_endpoints"), Description: "List all CDN endpoints",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_certificates"), Description: "List all SSL/TLS certificates",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_registries"), Description: "List container registry repositories",
		Parameters: map[string]string{"registry_name": "Registry name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"registry_name"},
	},
	{
		Name: mcp.ToolName("digitalocean_list_tags"), Description: "List all tags",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
}
