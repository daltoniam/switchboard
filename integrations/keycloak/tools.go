package keycloak

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "keycloak_list_users",
		Description: "List and search Keycloak user identity accounts in the configured realm. Start here to find a username, email, user ID, or enabled/disabled account status. Returns selected profile fields only, not credentials or custom attributes. One bounded page, not exhaustive inventory.",
		Parameters:  map[string]string{"search": "Optional Keycloak username/name/email search, at most 512 characters", "first": "Zero-based offset, integer 0-1000000 (default 0)", "max": "Page size, integer 1-100 (default 20)"},
	},
	{
		Name:        "keycloak_get_user",
		Description: "Get selected Keycloak user profile and enabled/disabled account status by exact user ID. Use after keycloak_list_users. Excludes credentials, attributes, required actions, role mappings, and sessions; does not determine temporary brute-force lockout or group membership.",
		Parameters:  map[string]string{"user_id": "Exact literal Keycloak user ID from keycloak_list_users"},
		Required:    []string{"user_id"},
	},
	{
		Name:        "keycloak_list_groups",
		Description: "List or search Keycloak groups in the configured realm, returning group ID, name, path and optional subgroup count. One bounded non-recursive page; no group members, user membership, roles or attributes are retrieved.",
		Parameters:  map[string]string{"search": "Optional group name search, at most 512 characters", "first": "Zero-based offset, integer 0-1000000 (default 0)", "max": "Page size, integer 1-100 (default 20)"},
	},
	{
		Name:        "keycloak_list_events",
		Description: "List Keycloak login and login-error audit events for at most seven inclusive UTC calendar days. Use after keycloak_list_users to investigate failed logins for a user ID. Checks event persistence first; disabled persistence is an error, not an empty success. Excludes IP addresses, session IDs, event details, admin events, and non-login events. No claim of complete history or active collection.",
		Parameters:  map[string]string{"date_from": "Required inclusive UTC start date YYYY-MM-DD", "date_to": "Required inclusive UTC end date YYYY-MM-DD, within six days of date_from", "type": "Optional LOGIN or LOGIN_ERROR; omit for both", "user": "Optional exact user ID", "first": "Zero-based offset, integer 0-1000000 (default 0)", "max": "Page size, integer 1-100 (default 20)"},
		Required:    []string{"date_from", "date_to"},
	},
}
