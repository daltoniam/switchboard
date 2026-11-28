package aikido

import (
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

func zenAppID() param {
	return integer("app_id", "Zen app id (id from aikido_list_zen_apps).")
}

var endpointEcosystems = []string{"npm", "pypi", "vscode", "open_vsx", "maven", "nuget", "chrome", "golang", "skills_sh"}

var zenEndpoints = []endpoint{
	{
		tool: "aikido_list_zen_apps", method: http.MethodGet, path: "/firewall/apps", scope: "firewall:read", access: mcp.AccessRead,
		desc: "List Aikido Zen apps (in-app firewall, runtime protection and RASP) with environment, linked repository, token presence and whether blocking mode is on. Start here for Aikido Zen firewall attacks and blocking.",
	},
	{
		tool: "aikido_get_zen_app", method: http.MethodGet, path: "/firewall/apps/{app_id}", scope: "firewall:read", access: mcp.AccessRead,
		desc:   "Get one Aikido Zen firewall app with environment, instance count, linked repository and token hint.",
		params: ps(zenAppID()),
	},
	{
		tool: "aikido_get_zen_event", method: http.MethodGet, path: "/firewall/apps/{app_id}/events/{event_id}", scope: "firewall:read", access: mcp.AccessRead,
		desc: "Get one Aikido Zen firewall event (detected or blocked attack such as SQL injection, SSRF, path traversal): attack kind, payload, source, stack, request route, IP and user agent, instance and user.",
		params: ps(
			zenAppID(),
			integer("event_id", "Zen event id, for example from a zen.attack webhook."),
		),
	},
	{
		tool: "aikido_list_zen_users", method: http.MethodGet, path: "/firewall/apps/{app_id}/users", scope: "firewall:read", access: mcp.AccessRead,
		desc:   "List end users seen by an Aikido Zen app with last IP, location, first and last seen, and whether they are blocked.",
		params: ps(zenAppID()),
		unwrap: "users",
		page:   pages(1, 50, 20),
	},
	{
		tool: "aikido_get_zen_countries", method: http.MethodGet, path: "/firewall/apps/{app_id}/countries", scope: "firewall:read", access: mcp.AccessRead,
		desc:   "Get the country-based IP blocking (geo blocking) configuration of an Aikido Zen app.",
		params: ps(zenAppID()),
	},
	{
		tool: "aikido_get_zen_threat_lists", method: http.MethodGet, path: "/firewall/apps/{app_id}/ip-lists", scope: "firewall:read", access: mcp.AccessRead,
		desc:   "Get the threat IP list configuration of an Aikido Zen app: known threat actors and Tor exit node blocking mode.",
		params: ps(zenAppID()),
	},
	{
		tool: "aikido_get_zen_bot_lists", method: http.MethodGet, path: "/firewall/apps/{app_id}/bot-lists", scope: "firewall:read", access: mcp.AccessRead,
		desc:   "Get the bot blocking configuration (AI crawlers, scrapers and other bot lists) of an Aikido Zen app.",
		params: ps(zenAppID()),
	},
	{
		tool: "aikido_create_zen_app", method: http.MethodPost, path: "/firewall/apps", scope: "firewall:write", access: mcp.AccessWrite,
		desc: "Create an Aikido Zen firewall app. Generate its token in the Aikido app before deploying Zen.",
		params: ps(
			str("name", "App name.").req(),
			str("environment", "Environment.").req().oneOf("production", "staging", "development"),
			integer("repo_id", "Code repository id the app is built from."),
		),
	},
	{
		tool: "aikido_update_zen_app", method: http.MethodPut, path: "/firewall/apps/{app_id}", scope: "firewall:write", access: mcp.AccessWrite,
		desc: "Rename an Aikido Zen app or change its environment.",
		params: ps(
			zenAppID(),
			str("name", "App name.").req(),
			str("environment", "Environment.").req().oneOf("production", "staging", "development"),
		),
	},
	{
		tool: "aikido_update_zen_blocking", method: http.MethodPut, path: "/firewall/apps/{app_id}/blocking", scope: "firewall:write", access: mcp.AccessWrite,
		desc: "Turn Aikido Zen blocking mode on (block detected attacks) or off (monitor only) for an app.",
		params: ps(
			zenAppID(),
			boolean("block", "true blocks attacks, false only monitors.").req(),
			boolean("disable_minimum_wait_check", "Skip the 3-day monitoring period production apps need before blocking."),
		),
	},
	{
		tool: "aikido_block_zen_user", method: http.MethodPut, path: "/firewall/{app_id}/users/{user_id}", scope: "firewall:write", access: mcp.AccessWrite,
		desc: "Block or unblock one end user from an app protected by Aikido Zen.",
		params: ps(
			zenAppID(),
			str("user_id", "User id as passed to the Zen SDK (id from aikido_list_zen_users)."),
			boolean("block", "true blocks the user, false unblocks.").req(),
		),
	},
	{
		tool: "aikido_update_zen_ip_blocklist", method: http.MethodPut, path: "/firewall/apps/{app_id}/ip-blocklist", scope: "firewall:write", access: mcp.AccessDestructive,
		desc: "Replace the custom IP blocklist of an Aikido Zen app. The list replaces the current one entirely.",
		params: ps(
			zenAppID(),
			strs("ip_addresses", "Every IP address or CIDR range to block.").req(),
		),
	},
	{
		tool: "aikido_update_zen_countries", method: http.MethodPut, path: "/firewall/apps/{app_id}/countries", scope: "firewall:write", access: mcp.AccessWrite,
		desc: "Replace the country-based IP blocking of an Aikido Zen app: block the listed countries, or allow only them.",
		params: ps(
			zenAppID(),
			str("mode", "block blocks the listed countries; allow only allows them.").req().oneOf("block", "allow"),
			strs("list", "ISO country codes, for example [\"RU\", \"KP\"].").req(),
		),
	},
	{
		tool: "aikido_delete_zen_app", method: http.MethodDelete, path: "/firewall/apps/{app_id}", scope: "firewall:write", access: mcp.AccessDestructive,
		desc:   "Delete an Aikido Zen app; deployed Zen agents using its token stop reporting.",
		params: ps(zenAppID()),
	},
	{
		tool: "aikido_list_endpoint_devices", method: http.MethodGet, path: "/endpoint-protection/devices", scope: "endpoint_protection:read", access: mcp.AccessRead,
		desc: "List developer devices with Aikido endpoint (device) protection: OS, version, user, permission group, last heartbeat and status.",
	},
	{
		tool: "aikido_list_endpoint_installed_packages", method: http.MethodGet, path: "/endpoint-protection/installed-packages", scope: "endpoint_protection:read", access: mcp.AccessRead,
		desc: "List packages and extensions installed on Aikido-protected developer devices, with the devices that have each one. Use to find which laptops installed a malicious package.",
		params: ps(
			str("filter_ecosystem", "Only this ecosystem.").oneOf("npm", "cargo", "pypi", "packagist", "golang", "maven", "nuget", "ruby", "open_vsx", "github_act", "vscode", "chrome", "wordpress", "skills_sh"),
			integer("filter_device", "Only this device id."),
			str("filter_package_name", "Only packages matching this name."),
		),
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_list_endpoint_activity", method: http.MethodGet, path: "/endpoint-protection/activityLogs", scope: "endpoint_protection:read", access: mcp.AccessRead,
		desc:   "List Aikido endpoint protection activity (blocked or allowed package installs) with device, package, version and time.",
		params: ps(integer("after_timestamp", "Only activity after this Unix timestamp (seconds).")),
		page:   pages(1, 50, 20),
	},
	{
		tool: "aikido_list_endpoint_exceptions", method: http.MethodGet, path: "/endpoint-protection/{ecosystem}/exceptions", scope: "endpoint_protection:read", access: mcp.AccessRead,
		desc:   "List Aikido endpoint protection package exceptions (allowed or blocked packages) for one ecosystem.",
		params: ps(str("ecosystem", "Package ecosystem.").oneOf(endpointEcosystems...)),
	},
	{
		tool: "aikido_list_endpoint_permission_groups", method: http.MethodGet, path: "/endpoint-protection/permission-groups", scope: "endpoint_protection:read", access: mcp.AccessRead,
		desc: "List Aikido endpoint protection permission groups with device counts.",
	},
	{
		tool: "aikido_add_endpoint_exception", method: http.MethodPost, path: "/endpoint-protection/{ecosystem}/exceptions", scope: "endpoint_protection:write", access: mcp.AccessWrite,
		desc: "Allow or block a package on Aikido-protected developer devices for one ecosystem.",
		params: ps(
			str("ecosystem", "Package ecosystem.").oneOf(endpointEcosystems...),
			str("package_name", "Package name.").req(),
			str("status", "allowed or blocked.").req().oneOf("allowed", "blocked"),
			integer("permission_group_id", "Limit to this permission group id."),
			boolean("bypass_default_minimum_allowed_age", "Allow installs newer than the default minimum package age (allowed only)."),
		),
	},
	{
		tool: "aikido_remove_endpoint_exception", method: http.MethodDelete, path: "/endpoint-protection/exceptions/{package_exception_id}", scope: "endpoint_protection:write", access: mcp.AccessWrite,
		desc:   "Remove an Aikido endpoint protection package exception.",
		params: ps(integer("package_exception_id", "Exception id from aikido_list_endpoint_exceptions.")),
	},
	{
		tool: "aikido_remove_endpoint_device", method: http.MethodDelete, path: "/endpoint-protection/devices/{device_id}", scope: "endpoint_protection:write", access: mcp.AccessDestructive,
		desc:   "Remove a developer device from Aikido endpoint protection.",
		params: ps(integer("device_id", "Device id from aikido_list_endpoint_devices.")),
	},
}
