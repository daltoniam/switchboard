package okta

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("okta_list_users"), Description: "List Okta identity directory users, employees, and accounts. Start here for IAM, SSO, provisioning, access, and user lookup workflows.",
		Parameters: map[string]string{
			"q":      "Simple search query matching firstName, lastName, or email",
			"filter": "SCIM filter (e.g. status eq \"ACTIVE\")",
			"search": "Advanced search expression (e.g. profile.email eq \"jane@acme.com\")",
			"after":  "Pagination cursor from next_after in a previous list response",
			"limit":  "Page size (default 20, max 200)",
		},
	},
	{
		Name: mcp.ToolName("okta_get_user"), Description: "Get an Okta user, employee, or account by ID or login email. Use after list_users.",
		Parameters: map[string]string{"user_id": "User ID or login (email)"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_create_user"), Description: "Create (add, provision) an Okta identity directory user or employee account",
		Parameters: map[string]string{
			"profile":   `JSON profile object. Required keys: firstName, lastName, email, login (e.g. {"firstName":"Jane","lastName":"Doe","email":"jane@acme.com","login":"jane@acme.com"})`,
			"activate":  "If true, activate immediately (default true)",
			"group_ids": "Optional JSON array of group IDs to add the user to",
		},
		Required: []string{"profile"},
	},
	{
		Name: mcp.ToolName("okta_update_user"), Description: "Update (edit, modify) an Okta user profile. Partial update via POST.",
		Parameters: map[string]string{
			"user_id": "User ID or login",
			"profile": "JSON profile object with fields to change",
		},
		Required: []string{"user_id", "profile"},
	},
	{
		Name: mcp.ToolName("okta_activate_user"), Description: "Activate a staged or provisioned Okta user so they can sign in. Use after create_user or list_users.",
		Parameters: map[string]string{
			"user_id":    "User ID or login",
			"send_email": "If true, send activation email (default true)",
		},
		Required: []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_deactivate_user"), Description: "Deactivate (disable) an Okta user account. Use after list_users or get_user.",
		Parameters: map[string]string{
			"user_id":    "User ID or login",
			"send_email": "If true, send deactivation email (default false)",
		},
		Required: []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_suspend_user"), Description: "Suspend an Okta user so they cannot sign in. Use after list_users or get_user.",
		Parameters: map[string]string{"user_id": "User ID or login"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_unsuspend_user"), Description: "Unsuspend a previously suspended Okta user. Use after list_users or get_user.",
		Parameters: map[string]string{"user_id": "User ID or login"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_unlock_user"), Description: "Unlock a locked-out Okta user account after failed sign-in attempts",
		Parameters: map[string]string{"user_id": "User ID or login"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_reset_password"), Description: "Reset an Okta user password and optionally email a reset link",
		Parameters: map[string]string{
			"user_id":    "User ID or login",
			"send_email": "If true, send reset email (default true)",
		},
		Required: []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_expire_password"), Description: "Expire an Okta user password so they must change it at next login",
		Parameters: map[string]string{
			"user_id":       "User ID or login",
			"temp_password": "If true, return a temporary password",
		},
		Required: []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_list_user_groups"), Description: "List groups an Okta user belongs to. Use after get_user.",
		Parameters: map[string]string{
			"user_id": "User ID or login",
			"after":   "Pagination cursor",
			"limit":   "Page size (default 20, max 200)",
		},
		Required: []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_list_user_factors"), Description: "List MFA factors enrolled for an Okta user (Okta Verify, SMS, TOTP, WebAuthn)",
		Parameters: map[string]string{"user_id": "User ID or login"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("okta_list_groups"), Description: "List Okta groups used for access, SSO assignment, and membership",
		Parameters: map[string]string{
			"q":      "Search by group name",
			"filter": "SCIM filter (e.g. type eq \"OKTA_GROUP\")",
			"search": "Advanced search expression",
			"after":  "Pagination cursor from next_after",
			"limit":  "Page size (default 20, max 200)",
		},
	},
	{
		Name: mcp.ToolName("okta_get_group"), Description: "Get an Okta group by ID. Use after list_groups.",
		Parameters: map[string]string{"group_id": "Group ID"},
		Required:   []string{"group_id"},
	},
	{
		Name: mcp.ToolName("okta_create_group"), Description: "Create an Okta group for access and membership",
		Parameters: map[string]string{
			"name":        "Group name",
			"description": "Group description",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("okta_update_group"), Description: "Update an Okta group name or description. Use after list_groups or get_group.",
		Parameters: map[string]string{
			"group_id":    "Group ID",
			"name":        "New group name",
			"description": "New description",
		},
		Required: []string{"group_id", "name"},
	},
	{
		Name: mcp.ToolName("okta_delete_group"), Description: "Delete (remove) an Okta group. Use after list_groups or get_group.",
		Parameters: map[string]string{"group_id": "Group ID"},
		Required:   []string{"group_id"},
	},
	{
		Name: mcp.ToolName("okta_list_group_members"), Description: "List users who are members of an Okta group. Use after list_groups.",
		Parameters: map[string]string{
			"group_id": "Group ID",
			"after":    "Pagination cursor",
			"limit":    "Page size (default 20, max 200)",
		},
		Required: []string{"group_id"},
	},
	{
		Name: mcp.ToolName("okta_add_group_member"), Description: "Add a user to an Okta group (assign membership). Use after list_users and list_groups.",
		Parameters: map[string]string{
			"group_id": "Group ID",
			"user_id":  "User ID",
		},
		Required: []string{"group_id", "user_id"},
	},
	{
		Name: mcp.ToolName("okta_remove_group_member"), Description: "Remove a user from an Okta group. Use after list_group_members.",
		Parameters: map[string]string{
			"group_id": "Group ID",
			"user_id":  "User ID",
		},
		Required: []string{"group_id", "user_id"},
	},
	{
		Name: mcp.ToolName("okta_list_apps"), Description: "List Okta applications, SSO apps, and SAML/OIDC integrations assigned in the org",
		Parameters: map[string]string{
			"q":      "Search by app label or name",
			"filter": "SCIM filter (e.g. status eq \"ACTIVE\")",
			"after":  "Pagination cursor",
			"limit":  "Page size (default 20, max 200)",
		},
	},
	{
		Name: mcp.ToolName("okta_get_app"), Description: "Get an Okta application or SSO app by ID. Use after list_apps.",
		Parameters: map[string]string{"app_id": "Application ID"},
		Required:   []string{"app_id"},
	},
	{
		Name: mcp.ToolName("okta_list_app_users"), Description: "List users assigned to an Okta application. Use after list_apps.",
		Parameters: map[string]string{
			"app_id": "Application ID",
			"after":  "Pagination cursor",
			"limit":  "Page size (default 20, max 200)",
		},
		Required: []string{"app_id"},
	},
	{
		Name: mcp.ToolName("okta_assign_app_user"), Description: "Assign a user to an Okta application (grant SSO access). Use after list_apps and list_users.",
		Parameters: map[string]string{
			"app_id":  "Application ID",
			"user_id": "User ID",
			"profile": "Optional JSON app-user profile object",
		},
		Required: []string{"app_id", "user_id"},
	},
	{
		Name: mcp.ToolName("okta_unassign_app_user"), Description: "Unassign a user from an Okta application. Use after list_app_users.",
		Parameters: map[string]string{
			"app_id":  "Application ID",
			"user_id": "User ID",
		},
		Required: []string{"app_id", "user_id"},
	},
	{
		Name: mcp.ToolName("okta_list_app_groups"), Description: "List groups assigned to an Okta application. Use after list_apps.",
		Parameters: map[string]string{
			"app_id": "Application ID",
			"after":  "Pagination cursor",
			"limit":  "Page size (default 20, max 200)",
		},
		Required: []string{"app_id"},
	},
	{
		Name: mcp.ToolName("okta_assign_app_group"), Description: "Assign a group to an Okta application so members get SSO access. Use after list_apps and list_groups.",
		Parameters: map[string]string{
			"app_id":   "Application ID",
			"group_id": "Group ID",
		},
		Required: []string{"app_id", "group_id"},
	},
	{
		Name: mcp.ToolName("okta_unassign_app_group"), Description: "Unassign a group from an Okta application. Use after list_app_groups.",
		Parameters: map[string]string{
			"app_id":   "Application ID",
			"group_id": "Group ID",
		},
		Required: []string{"app_id", "group_id"},
	},
	{
		Name: mcp.ToolName("okta_list_policies"), Description: "List Okta access, password, MFA enroll, and sign-on policies",
		Parameters: map[string]string{
			"type": "Policy type: OKTA_SIGN_ON, PASSWORD, MFA_ENROLL, ACCESS_POLICY, PROFILE_ENROLLMENT, IDP_DISCOVERY",
		},
		Required: []string{"type"},
	},
	{
		Name: mcp.ToolName("okta_get_policy"), Description: "Get an Okta policy by ID. Use after list_policies.",
		Parameters: map[string]string{"policy_id": "Policy ID"},
		Required:   []string{"policy_id"},
	},
	{
		Name: mcp.ToolName("okta_list_policy_rules"), Description: "List rules for an Okta policy. Use after list_policies or get_policy.",
		Parameters: map[string]string{"policy_id": "Policy ID"},
		Required:   []string{"policy_id"},
	},
	{
		Name: mcp.ToolName("okta_list_logs"), Description: "Search Okta system log events for sign-in, authentication, audit, and security activity",
		Parameters: map[string]string{
			"since":  "Start time (ISO 8601)",
			"until":  "End time (ISO 8601)",
			"filter": "SCIM filter (e.g. eventType eq \"user.session.start\")",
			"q":      "Keyword search",
			"after":  "Pagination cursor from next_after",
			"limit":  "Page size (default 20, max 1000)",
		},
	},
	{
		Name:        mcp.ToolName("okta_get_org"),
		Description: "Get Okta organization (org, tenant) metadata including subdomain and company settings",
		Parameters:  map[string]string{},
	},
}
