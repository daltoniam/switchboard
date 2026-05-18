package fly

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Apps ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("fly_list_apps"), Description: "List all Fly.io apps in an organization. Start here for most workflows — returns app names needed by other tools",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("org_slug"), Description: "Organization slug (e.g. 'personal')", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_get_app"), Description: "Get details of a Fly.io app including status and organization info",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_create_app"), Description: "Create a new Fly.io app in an organization",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "Name for the new app", Required: true}, {Name: mcp.ParamName("org_slug"), Description: "Organization slug (e.g. 'personal')", Required: true}, {Name: mcp.ParamName("network"), Description: "Optional IPv6 private network name to segment the app onto"}},
	},
	{
		Name: mcp.ToolName("fly_delete_app"), Description: "Delete a Fly.io app. Use force=true to stop all Machines first",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name to delete", Required: true}, {Name: mcp.ParamName("force"), Description: "Force stop all Machines and delete immediately (true/false)"}},
	},

	// ── Machines ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("fly_list_machines"), Description: "List all Machines in a Fly.io app. Returns IDs, state, region, and image info. Use summary=true for lighter response",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("include_deleted"), Description: "Include deleted machines (true/false)"}, {Name: mcp.ParamName("region"), Description: "Filter by region code (e.g. 'ord', 'iad')"}, {Name: mcp.ParamName("state"), Description: "Comma-separated states to filter: created, started, stopped, suspended"}, {Name: mcp.ParamName("summary"), Description: "Only return summary info, omit config/checks/events (true/false)"}},
	},
	{
		Name: mcp.ToolName("fly_get_machine"), Description: "Get full details of a specific Machine including config, events, and checks",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_create_machine"), Description: "Create a new Machine in a Fly.io app. Specify image, resources (guest), region, and optional services/mounts",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("name"), Description: "Optional machine name"}, {Name: mcp.ParamName("region"), Description: "Region code (e.g. 'ord', 'iad', 'lhr')"}, {Name: mcp.ParamName("config"), Description: "Machine config object with: image (required), guest {cpus, memory_mb, cpu_kind}, env {}, services [], mounts [], auto_destroy, restart {policy}, metadata {}", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_update_machine"), Description: "Update a Machine's configuration (image, resources, env, services, etc.)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}, {Name: mcp.ParamName("config"), Description: "Updated machine config object", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_delete_machine"), Description: "Delete a Machine. Use force=true to kill a running machine",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}, {Name: mcp.ParamName("force"), Description: "Force kill if running (true/false)"}},
	},
	{
		Name: mcp.ToolName("fly_start_machine"), Description: "Start a stopped Machine",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_stop_machine"), Description: "Stop a running Machine",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_restart_machine"), Description: "Restart a Machine",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_signal_machine"), Description: "Send a Unix signal to a Machine process (e.g. SIGTERM, SIGKILL, SIGHUP)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}, {Name: mcp.ParamName("signal"), Description: "Signal name (SIGTERM, SIGKILL, SIGHUP, SIGUSR1, SIGUSR2, etc.)", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_wait_machine"), Description: "Wait for a Machine to reach a specific state. Blocks until state is reached or timeout",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}, {Name: mcp.ParamName("state"), Description: "Target state: started, stopped, suspended, destroyed (default: started)"}, {Name: mcp.ParamName("timeout"), Description: "Timeout in seconds (default: 60)"}},
	},
	{
		Name: mcp.ToolName("fly_exec_machine"), Description: "Execute a command inside a running Machine and return stdout/stderr",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("machine_id"), Description: "Machine ID", Required: true}, {Name: mcp.ParamName("command"), Description: `Command to execute as an array of strings (e.g. ["ls", "-la"])`,

		// ── Volumes ──────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("fly_list_volumes"), Description: "List all persistent volumes attached to a Fly.io app",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_get_volume"), Description: "Get details of a specific volume including size, region, and attached machine",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("volume_id"), Description: "Volume ID", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_create_volume"), Description: "Create a persistent volume in a Fly.io app. Volumes are region-specific",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("name"), Description: "Volume name", Required: true}, {Name: mcp.ParamName("region"), Description: "Region code (e.g. 'ord')", Required: true}, {Name: mcp.ParamName("size_gb"), Description: "Size in GB (default: 1)"}, {Name: mcp.ParamName("encrypted"), Description: "Encrypt the volume (true/false, default: true)"}, {Name: mcp.ParamName("snapshot_retention"), Description: "Number of snapshots to retain"}, {Name: mcp.ParamName("auto_backup_enabled"), Description: "Enable automatic backups (true/false)"}},
	},
	{
		Name: mcp.ToolName("fly_update_volume"), Description: "Update a volume's size or snapshot settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("volume_id"), Description: "Volume ID", Required: true}, {Name: mcp.ParamName("snapshot_retention"), Description: "Number of snapshots to retain"}, {Name: mcp.ParamName("auto_backup_enabled"), Description: "Enable automatic backups (true/false)"}},
	},
	{
		Name: mcp.ToolName("fly_delete_volume"), Description: "Delete a persistent volume. Volume must not be attached to a running machine",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("volume_id"), Description: "Volume ID", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_list_volume_snapshots"), Description: "List snapshots for a specific volume",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("volume_id"), Description: "Volume ID",

		// ── Secrets ──────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("fly_list_secrets"), Description: "List all secrets for a Fly.io app. Returns names and digests only — values are never exposed",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}},
	},
	{
		Name: mcp.ToolName("fly_set_secrets"), Description: "Set one or more secrets on a Fly.io app. Machines will be restarted to pick up changes",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("secrets"), Description: `Key-value map of secrets to set (e.g. {"DATABASE_URL": "postgres://..."})`, Required: true}},
	},
	{
		Name: mcp.ToolName("fly_unset_secrets"), Description: "Remove one or more secrets from a Fly.io app. Machines will be restarted",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("app_name"), Description: "App name", Required: true}, {Name: mcp.ParamName("keys"), Description: "Array of secret names to remove", Required: true}},
	},
}
