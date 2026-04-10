package digitalocean

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Account ─────────────────────────────────────────────────────
	{
		Name: "digitalocean_get_account", Description: "Get current account information including email, droplet limit, and status",
		Parameters: map[string]string{},
	},

	// ── Droplets ────────────────────────────────────────────────────
	{
		Name: "digitalocean_list_droplets", Description: "List all droplets in the account",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page (max 200)", "tag_name": "Filter by tag"},
	},
	{
		Name: "digitalocean_get_droplet", Description: "Get details of a specific droplet",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: "digitalocean_create_droplet", Description: "Create a new droplet",
		Parameters: map[string]string{"name": "Droplet name", "region": "Region slug (e.g. nyc3)", "size": "Size slug (e.g. s-1vcpu-1gb)", "image": "Image slug or ID (e.g. ubuntu-24-04-x64)", "ssh_keys": "Comma-separated SSH key IDs or fingerprints", "tags": "Comma-separated tags", "vpc_uuid": "VPC UUID"},
		Required:   []string{"name", "region", "size", "image"},
	},
	{
		Name: "digitalocean_delete_droplet", Description: "Delete a droplet by ID",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: "digitalocean_reboot_droplet", Description: "Reboot a droplet",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: "digitalocean_poweroff_droplet", Description: "Power off a droplet (hard shutdown)",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},
	{
		Name: "digitalocean_poweron_droplet", Description: "Power on a droplet",
		Parameters: map[string]string{"droplet_id": "Droplet ID (integer)"},
		Required:   []string{"droplet_id"},
	},

	// ── Kubernetes ──────────────────────────────────────────────────
	{
		Name: "digitalocean_list_kubernetes_clusters", Description: "List all Kubernetes clusters",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_kubernetes_cluster", Description: "Get details of a specific Kubernetes cluster",
		Parameters: map[string]string{"cluster_id": "Cluster UUID"},
		Required:   []string{"cluster_id"},
	},
	{
		Name: "digitalocean_list_kubernetes_node_pools", Description: "List node pools for a Kubernetes cluster",
		Parameters: map[string]string{"cluster_id": "Cluster UUID"},
		Required:   []string{"cluster_id"},
	},

	// ── Databases ───────────────────────────────────────────────────
	{
		Name: "digitalocean_list_databases", Description: "List all managed database clusters",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_database", Description: "Get details of a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},
	{
		Name: "digitalocean_list_database_dbs", Description: "List databases within a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},
	{
		Name: "digitalocean_list_database_users", Description: "List users for a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},
	{
		Name: "digitalocean_list_database_pools", Description: "List connection pools for a managed database cluster",
		Parameters: map[string]string{"database_id": "Database cluster UUID"},
		Required:   []string{"database_id"},
	},

	// ── Networking ──────────────────────────────────────────────────
	{
		Name: "digitalocean_list_domains", Description: "List all domains",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_domain", Description: "Get details of a domain",
		Parameters: map[string]string{"domain_name": "Domain name (e.g. example.com)"},
		Required:   []string{"domain_name"},
	},
	{
		Name: "digitalocean_list_domain_records", Description: "List DNS records for a domain",
		Parameters: map[string]string{"domain_name": "Domain name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"domain_name"},
	},
	{
		Name: "digitalocean_list_load_balancers", Description: "List all load balancers",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_load_balancer", Description: "Get details of a load balancer",
		Parameters: map[string]string{"lb_id": "Load balancer UUID"},
		Required:   []string{"lb_id"},
	},
	{
		Name: "digitalocean_list_firewalls", Description: "List all cloud firewalls",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_firewall", Description: "Get details of a cloud firewall",
		Parameters: map[string]string{"firewall_id": "Firewall UUID"},
		Required:   []string{"firewall_id"},
	},
	{
		Name: "digitalocean_list_vpcs", Description: "List all VPCs",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_vpc", Description: "Get details of a VPC",
		Parameters: map[string]string{"vpc_id": "VPC UUID"},
		Required:   []string{"vpc_id"},
	},

	// ── Volumes ─────────────────────────────────────────────────────
	{
		Name: "digitalocean_list_volumes", Description: "List all block storage volumes",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page", "region": "Filter by region slug"},
	},
	{
		Name: "digitalocean_get_volume", Description: "Get details of a block storage volume",
		Parameters: map[string]string{"volume_id": "Volume UUID"},
		Required:   []string{"volume_id"},
	},

	// ── Apps ────────────────────────────────────────────────────────
	{
		Name: "digitalocean_list_apps", Description: "List all App Platform apps",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_app", Description: "Get details of an App Platform app including its spec, active deployment, and domain info",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},
	{
		Name: "digitalocean_create_app", Description: "Create a new App Platform app from a JSON spec. The spec defines services, workers, jobs, databases, and static sites",
		Parameters: map[string]string{"spec": "App spec as JSON string (see DO App Spec reference)", "project_id": "Optional project UUID to assign the app to"},
		Required:   []string{"spec"},
	},
	{
		Name: "digitalocean_update_app", Description: "Update an App Platform app's spec. Triggers a new deployment with the updated configuration",
		Parameters: map[string]string{"app_id": "App UUID", "spec": "Updated app spec as JSON string"},
		Required:   []string{"app_id", "spec"},
	},
	{
		Name: "digitalocean_delete_app", Description: "Delete an App Platform app and all its resources",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},
	{
		Name: "digitalocean_restart_app", Description: "Restart an App Platform app. Optionally restart only specific components",
		Parameters: map[string]string{"app_id": "App UUID", "components": "Comma-separated component names to restart (omit to restart all)"},
		Required:   []string{"app_id"},
	},
	{
		Name: "digitalocean_list_app_deployments", Description: "List deployment history for an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"app_id"},
	},
	{
		Name: "digitalocean_get_app_deployment", Description: "Get details of a specific deployment including phase, progress, and timing",
		Parameters: map[string]string{"app_id": "App UUID", "deployment_id": "Deployment UUID"},
		Required:   []string{"app_id", "deployment_id"},
	},
	{
		Name: "digitalocean_create_app_deployment", Description: "Trigger a new deployment for an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID", "force_build": "Force rebuild even if no changes detected (true/false)"},
		Required:   []string{"app_id"},
	},
	{
		Name: "digitalocean_cancel_app_deployment", Description: "Cancel an in-progress deployment",
		Parameters: map[string]string{"app_id": "App UUID", "deployment_id": "Deployment UUID"},
		Required:   []string{"app_id", "deployment_id"},
	},
	{
		Name: "digitalocean_get_app_logs", Description: "Get logs for a specific deployment. Use digitalocean_list_app_deployments to find deployment IDs",
		Parameters: map[string]string{"app_id": "App UUID", "deployment_id": "Deployment UUID", "component": "Component name to filter logs", "log_type": "Log type: BUILD, DEPLOY, or RUN (defaults to RUN)"},
		Required:   []string{"app_id", "deployment_id"},
	},
	{
		Name: "digitalocean_get_app_health", Description: "Get health status of an App Platform app and its components",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},
	{
		Name: "digitalocean_list_app_alerts", Description: "List all configured alerts for an App Platform app",
		Parameters: map[string]string{"app_id": "App UUID"},
		Required:   []string{"app_id"},
	},
	{
		Name: "digitalocean_rollback_app", Description: "Rollback an App Platform app to a previous deployment",
		Parameters: map[string]string{"app_id": "App UUID", "deployment_id": "Deployment UUID to rollback to"},
		Required:   []string{"app_id", "deployment_id"},
	},

	// ── Extras ──────────────────────────────────────────────────────
	{
		Name: "digitalocean_list_regions", Description: "List all available regions",
		Parameters: map[string]string{},
	},
	{
		Name: "digitalocean_list_sizes", Description: "List all available droplet sizes",
		Parameters: map[string]string{},
	},
	{
		Name: "digitalocean_list_images", Description: "List available images (OS distributions and snapshots)",
		Parameters: map[string]string{"type": "Filter by type: distribution, application, or user", "page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_list_ssh_keys", Description: "List all SSH keys in the account",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_list_snapshots", Description: "List all snapshots (droplet and volume)",
		Parameters: map[string]string{"resource_type": "Filter: droplet or volume", "page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_list_projects", Description: "List all projects",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_get_project", Description: "Get details of a project",
		Parameters: map[string]string{"project_id": "Project UUID"},
		Required:   []string{"project_id"},
	},
	{
		Name: "digitalocean_get_balance", Description: "Get current account balance",
		Parameters: map[string]string{},
	},
	{
		Name: "digitalocean_list_invoices", Description: "List billing invoices",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_list_cdn_endpoints", Description: "List all CDN endpoints",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_list_certificates", Description: "List all SSL/TLS certificates",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
	{
		Name: "digitalocean_list_registries", Description: "List container registry repositories",
		Parameters: map[string]string{"registry_name": "Registry name", "page": "Page number", "per_page": "Results per page"},
		Required:   []string{"registry_name"},
	},
	{
		Name: "digitalocean_list_tags", Description: "List all tags",
		Parameters: map[string]string{"page": "Page number", "per_page": "Results per page"},
	},
}
