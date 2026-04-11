package fly

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Apps ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("fly_list_apps"), Description: "List all Fly.io apps in an organization. Start here for most workflows — returns app names needed by other tools",
		Parameters: map[string]string{"org_slug": "Organization slug (e.g. 'personal')"},
		Required:   []string{"org_slug"},
	},
	{
		Name: mcp.ToolName("fly_get_app"), Description: "Get details of a Fly.io app including status and organization info",
		Parameters: map[string]string{"app_name": "App name"},
		Required:   []string{"app_name"},
	},
	{
		Name: mcp.ToolName("fly_create_app"), Description: "Create a new Fly.io app in an organization",
		Parameters: map[string]string{
			"app_name": "Name for the new app",
			"org_slug": "Organization slug (e.g. 'personal')",
			"network":  "Optional IPv6 private network name to segment the app onto",
		},
		Required: []string{"app_name", "org_slug"},
	},
	{
		Name: mcp.ToolName("fly_delete_app"), Description: "Delete a Fly.io app. Use force=true to stop all Machines first",
		Parameters: map[string]string{
			"app_name": "App name to delete",
			"force":    "Force stop all Machines and delete immediately (true/false)",
		},
		Required: []string{"app_name"},
	},

	// ── Machines ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("fly_list_machines"), Description: "List all Machines in a Fly.io app. Returns IDs, state, region, and image info. Use summary=true for lighter response",
		Parameters: map[string]string{
			"app_name":        "App name",
			"include_deleted": "Include deleted machines (true/false)",
			"region":          "Filter by region code (e.g. 'ord', 'iad')",
			"state":           "Comma-separated states to filter: created, started, stopped, suspended",
			"summary":         "Only return summary info, omit config/checks/events (true/false)",
		},
		Required: []string{"app_name"},
	},
	{
		Name: mcp.ToolName("fly_get_machine"), Description: "Get full details of a specific Machine including config, events, and checks",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
		},
		Required: []string{"app_name", "machine_id"},
	},
	{
		Name: mcp.ToolName("fly_create_machine"), Description: "Create a new Machine in a Fly.io app. Specify image, resources (guest), region, and optional services/mounts",
		Parameters: map[string]string{
			"app_name": "App name",
			"name":     "Optional machine name",
			"region":   "Region code (e.g. 'ord', 'iad', 'lhr')",
			"config":   "Machine config object with: image (required), guest {cpus, memory_mb, cpu_kind}, env {}, services [], mounts [], auto_destroy, restart {policy}, metadata {}",
		},
		Required: []string{"app_name", "config"},
	},
	{
		Name: mcp.ToolName("fly_update_machine"), Description: "Update a Machine's configuration (image, resources, env, services, etc.)",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
			"config":     "Updated machine config object",
		},
		Required: []string{"app_name", "machine_id", "config"},
	},
	{
		Name: mcp.ToolName("fly_delete_machine"), Description: "Delete a Machine. Use force=true to kill a running machine",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
			"force":      "Force kill if running (true/false)",
		},
		Required: []string{"app_name", "machine_id"},
	},
	{
		Name: mcp.ToolName("fly_start_machine"), Description: "Start a stopped Machine",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
		},
		Required: []string{"app_name", "machine_id"},
	},
	{
		Name: mcp.ToolName("fly_stop_machine"), Description: "Stop a running Machine",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
		},
		Required: []string{"app_name", "machine_id"},
	},
	{
		Name: mcp.ToolName("fly_restart_machine"), Description: "Restart a Machine",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
		},
		Required: []string{"app_name", "machine_id"},
	},
	{
		Name: mcp.ToolName("fly_signal_machine"), Description: "Send a Unix signal to a Machine process (e.g. SIGTERM, SIGKILL, SIGHUP)",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
			"signal":     "Signal name (SIGTERM, SIGKILL, SIGHUP, SIGUSR1, SIGUSR2, etc.)",
		},
		Required: []string{"app_name", "machine_id", "signal"},
	},
	{
		Name: mcp.ToolName("fly_wait_machine"), Description: "Wait for a Machine to reach a specific state. Blocks until state is reached or timeout",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
			"state":      "Target state: started, stopped, suspended, destroyed (default: started)",
			"timeout":    "Timeout in seconds (default: 60)",
		},
		Required: []string{"app_name", "machine_id"},
	},
	{
		Name: mcp.ToolName("fly_exec_machine"), Description: "Execute a command inside a running Machine and return stdout/stderr",
		Parameters: map[string]string{
			"app_name":   "App name",
			"machine_id": "Machine ID",
			"command":    "Command to execute as an array of strings (e.g. [\"ls\", \"-la\"])",
		},
		Required: []string{"app_name", "machine_id", "command"},
	},

	// ── Volumes ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("fly_list_volumes"), Description: "List all persistent volumes attached to a Fly.io app",
		Parameters: map[string]string{"app_name": "App name"},
		Required:   []string{"app_name"},
	},
	{
		Name: mcp.ToolName("fly_get_volume"), Description: "Get details of a specific volume including size, region, and attached machine",
		Parameters: map[string]string{
			"app_name":  "App name",
			"volume_id": "Volume ID",
		},
		Required: []string{"app_name", "volume_id"},
	},
	{
		Name: mcp.ToolName("fly_create_volume"), Description: "Create a persistent volume in a Fly.io app. Volumes are region-specific",
		Parameters: map[string]string{
			"app_name":            "App name",
			"name":                "Volume name",
			"region":              "Region code (e.g. 'ord')",
			"size_gb":             "Size in GB (default: 1)",
			"encrypted":           "Encrypt the volume (true/false, default: true)",
			"snapshot_retention":  "Number of snapshots to retain",
			"auto_backup_enabled": "Enable automatic backups (true/false)",
		},
		Required: []string{"app_name", "name", "region"},
	},
	{
		Name: mcp.ToolName("fly_update_volume"), Description: "Update a volume's size or snapshot settings",
		Parameters: map[string]string{
			"app_name":            "App name",
			"volume_id":           "Volume ID",
			"snapshot_retention":  "Number of snapshots to retain",
			"auto_backup_enabled": "Enable automatic backups (true/false)",
		},
		Required: []string{"app_name", "volume_id"},
	},
	{
		Name: mcp.ToolName("fly_delete_volume"), Description: "Delete a persistent volume. Volume must not be attached to a running machine",
		Parameters: map[string]string{
			"app_name":  "App name",
			"volume_id": "Volume ID",
		},
		Required: []string{"app_name", "volume_id"},
	},
	{
		Name: mcp.ToolName("fly_list_volume_snapshots"), Description: "List snapshots for a specific volume",
		Parameters: map[string]string{
			"app_name":  "App name",
			"volume_id": "Volume ID",
		},
		Required: []string{"app_name", "volume_id"},
	},

	// ── Secrets ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("fly_list_secrets"), Description: "List all secrets for a Fly.io app. Returns names and digests only — values are never exposed",
		Parameters: map[string]string{"app_name": "App name"},
		Required:   []string{"app_name"},
	},
	{
		Name: mcp.ToolName("fly_set_secrets"), Description: "Set one or more secrets on a Fly.io app. Machines will be restarted to pick up changes",
		Parameters: map[string]string{
			"app_name": "App name",
			"secrets":  "Key-value map of secrets to set (e.g. {\"DATABASE_URL\": \"postgres://...\"})",
		},
		Required: []string{"app_name", "secrets"},
	},
	{
		Name: mcp.ToolName("fly_unset_secrets"), Description: "Remove one or more secrets from a Fly.io app. Machines will be restarted",
		Parameters: map[string]string{
			"app_name": "App name",
			"keys":     "Array of secret names to remove",
		},
		Required: []string{"app_name", "keys"},
	},
}
