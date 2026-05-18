package switchboard

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "switchboard_list_integrations",
		Description: "List all registered integrations with their enabled/healthy status, tool counts, and credential keys. " +
			"Start here for Switchboard self-management, configuration, and integration setup. " +
			"Shows which integrations are configured, which need credentials, and which are healthy.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("enabled_only"), Description: "If true, only show enabled integrations (default: false)."}},
	},
	{
		Name: "switchboard_get_integration",
		Description: "Get detailed information about a specific integration including credential keys (not values), " +
			"tool list, enabled/healthy status, and configuration hints. " +
			"Use after list_integrations to inspect a single integration before configuring it.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: `Integration name (e.g. "github", "datadog", "slack").`, Required: true}},
	},
	{
		Name: "switchboard_configure_integration",
		Description: "Configure an integration by setting credentials and enabling or disabling it. " +
			"Credentials are merged with existing values — send only the keys you want to update. " +
			"Set enabled=false to disable an integration without removing its credentials.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: `Integration name (e.g. "github", "datadog").`, Required: true}, {Name: mcp.ParamName("credentials"), Description: "JSON object of credential key-value pairs to set (merged with existing)."}, {Name: mcp.ParamName("enabled"), Description: "Whether to enable the integration after configuring (default: true)."}},
	},
	{
		Name: "switchboard_check_health",
		Description: "Check connectivity health of one or all enabled integrations. " +
			"Returns healthy/unhealthy status for each integration by calling its health endpoint. " +
			"Use to verify credentials work and upstream APIs are reachable.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Optional: specific integration to check. Omit to check all enabled integrations."}},
	},
	{
		Name: "switchboard_browse_plugins",
		Description: "Browse available plugins from configured marketplace manifest sources. " +
			"Shows plugin name, description, latest version, and whether it is already installed. " +
			"Use before install_plugin to discover available extensions.",
		Parameters: []mcp.Parameter{},
	},
	{
		Name: "switchboard_install_plugin",
		Description: "Install a plugin from the marketplace by name or from a direct URL. " +
			"Downloads the WASM module, verifies its checksum, and registers it. " +
			"Requires a server restart to load the plugin. Use after browse_plugins.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Plugin name from the marketplace (mutually exclusive with url)."}, {Name: mcp.ParamName("url"), Description: "Direct URL to a .wasm file to install (mutually exclusive with name)."}},
	},
	{
		Name: "switchboard_uninstall_plugin",
		Description: "Uninstall a marketplace plugin by name. Removes the WASM file and " +
			"deregisters it from config. Requires a server restart to take effect.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Name of the installed plugin to remove.", Required: true}},
	},
	{
		Name: "switchboard_server_info",
		Description: "Get Switchboard server information including version, enabled integration count, " +
			"total tool count, marketplace status, and runtime metrics summary. " +
			"Use for diagnostics, status checks, and understanding the current server state.",
		Parameters: []mcp.Parameter{},
	},
}
