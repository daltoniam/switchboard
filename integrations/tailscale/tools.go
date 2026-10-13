package tailscale

import mcp "github.com/daltoniam/switchboard"

const (
	deviceIDParam = "Device ID from list_devices (the nodeId like nXXXXCNTRL or the numeric id)"
	userIDParam   = "Tailscale user ID from list_users"
)

var tools = []mcp.ToolDefinition{
	// Devices
	{
		Name:        mcp.ToolName("tailscale_list_devices"),
		Description: "List Tailscale devices (machines, nodes, servers) in the tailnet VPN with hostname, Tailscale IP addresses, OS, owner, tags, online/last seen, authorization, and key expiry. Start here for Tailscale network management, finding a device_id, checking which machines are offline or unauthorized, and auditing the tailnet.",
		Parameters: map[string]string{
			"fields":   "default (core fields) or all (adds advertised/enabled subnet routes, client connectivity, posture). Default: default",
			"hostname": "Optional exact hostname filter",
			"tags":     "Optional tag filter list (e.g. [\"tag:prod\"]); devices must have every tag",
			"os":       "Optional exact OS filter (linux, macOS, windows, iOS, android)",
		},
	},
	{
		Name:        mcp.ToolName("tailscale_get_device"),
		Description: "Get full details of one Tailscale device (machine, node) including IPs, routes, tags, client version, connectivity, posture, and key expiry. Use after list_devices.",
		Parameters: map[string]string{
			"device_id": deviceIDParam,
			"fields":    "all (default) or default",
		},
		Required: []string{"device_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_authorize_device"),
		Description: "Approve (authorize) or deauthorize a Tailscale device waiting for device approval so it can join the tailnet. Use after list_devices shows authorized=false.",
		Parameters: map[string]string{
			"device_id":  deviceIDParam,
			"authorized": "true to approve (default), false to revoke authorization",
		},
		Required: []string{"device_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_delete_device"),
		Description: "Delete (remove) a Tailscale device (machine, node) from the tailnet. Destructive: the device must re-authenticate to rejoin. Use after list_devices to confirm the device_id.",
		Parameters:  map[string]string{"device_id": deviceIDParam},
		Required:    []string{"device_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_device_name"),
		Description: "Rename a Tailscale device, changing its MagicDNS machine name. Use after list_devices.",
		Parameters: map[string]string{
			"device_id": deviceIDParam,
			"name":      "New machine name (short name or full MagicDNS name)",
		},
		Required: []string{"device_id", "name"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_device_tags"),
		Description: "Replace the ACL tags on a Tailscale device (e.g. tag:server, tag:prod). Tags change ownership and which ACL rules apply. Pass an empty list to remove all tags. Use after list_devices.",
		Parameters: map[string]string{
			"device_id": deviceIDParam,
			"tags":      "Full list of tags to set (each prefixed tag:); replaces existing tags",
		},
		Required: []string{"device_id", "tags"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_device_key_expiry"),
		Description: "Disable or re-enable node key expiry for a Tailscale device (keep servers from expiring and dropping off the tailnet). Use after list_devices.",
		Parameters: map[string]string{
			"device_id":           deviceIDParam,
			"key_expiry_disabled": "true to disable key expiry, false to re-enable it",
		},
		Required: []string{"device_id", "key_expiry_disabled"},
	},
	{
		Name:        mcp.ToolName("tailscale_expire_device_key"),
		Description: "Expire a Tailscale device's node key immediately, forcing the machine to re-authenticate. Use to revoke access for a lost or compromised device.",
		Parameters:  map[string]string{"device_id": deviceIDParam},
		Required:    []string{"device_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_device_ip"),
		Description: "Change a Tailscale device's Tailscale IPv4 address (100.x.y.z). Breaks existing connections to that machine.",
		Parameters: map[string]string{
			"device_id": deviceIDParam,
			"ipv4":      "New Tailscale IPv4 address within the CGNAT range (e.g. 100.80.0.1)",
		},
		Required: []string{"device_id", "ipv4"},
	},
	{
		Name:        mcp.ToolName("tailscale_get_device_routes"),
		Description: "Get subnet routes and exit node routes a Tailscale device advertises versus which are enabled (approved). Use to find subnet routers or exit nodes waiting for route approval.",
		Parameters:  map[string]string{"device_id": deviceIDParam},
		Required:    []string{"device_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_device_routes"),
		Description: "Enable (approve) subnet routes or an exit node on a Tailscale device by replacing its enabled routes list. Use 0.0.0.0/0 and ::/0 to approve an exit node; pass an empty list to disable all routes. Use after get_device_routes.",
		Parameters: map[string]string{
			"device_id": deviceIDParam,
			"routes":    "Full list of CIDR routes to enable (e.g. [\"10.0.0.0/16\"]); replaces existing enabled routes",
		},
		Required: []string{"device_id", "routes"},
	},
	{
		Name:        mcp.ToolName("tailscale_get_device_posture_attributes"),
		Description: "Get device posture attributes for a Tailscale device (node:os, node:tsVersion, custom: attributes, MDM/EDR integration data) used by posture-based ACL conditions.",
		Parameters:  map[string]string{"device_id": deviceIDParam},
		Required:    []string{"device_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_device_posture_attribute"),
		Description: "Create or update a custom device posture attribute (custom:*) on a Tailscale device for posture-based access control. Use after get_device_posture_attributes.",
		Parameters: map[string]string{
			"device_id":     deviceIDParam,
			"attribute_key": "Attribute key, must start with custom: (e.g. custom:tier)",
			"value":         "String (letters, numbers, underscores, periods; max 50 chars), integer, or boolean",
			"expiry":        "Optional RFC3339 time after which the attribute is removed",
			"comment":       "Optional audit log comment (max 200 chars)",
		},
		Required: []string{"device_id", "attribute_key", "value"},
	},
	{
		Name:        mcp.ToolName("tailscale_delete_device_posture_attribute"),
		Description: "Delete a custom device posture attribute (custom:*) from a Tailscale device.",
		Parameters: map[string]string{
			"device_id":     deviceIDParam,
			"attribute_key": "Attribute key, must start with custom:",
		},
		Required: []string{"device_id", "attribute_key"},
	},

	// Keys
	{
		Name:        mcp.ToolName("tailscale_list_keys"),
		Description: "List Tailscale auth keys, API access tokens, and OAuth clients (trust credentials) with expiry, description, and capabilities. Secret values are never returned. Use to audit or find a key_id to revoke.",
		Parameters: map[string]string{
			"all": "true to list keys for all users in the tailnet rather than only the caller's own keys",
		},
	},
	{
		Name:        mcp.ToolName("tailscale_get_key"),
		Description: "Get one Tailscale auth key or API token by key_id, including expiry, revocation, tags, and reusable/ephemeral/preauthorized capabilities. Use after list_keys.",
		Parameters:  map[string]string{"key_id": "Key ID from list_keys"},
		Required:    []string{"key_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_create_auth_key"),
		Description: "Create a Tailscale auth key (pre-auth key) for joining new devices, servers, containers, or CI runners to the tailnet. The secret key value is only returned once in this response. Tagged keys are recommended for servers.",
		Parameters: map[string]string{
			"reusable":       "true to allow multiple devices to use the key (default false: single use)",
			"ephemeral":      "true to auto-remove devices after they go offline (good for CI/containers)",
			"preauthorized":  "true to skip device approval for devices using this key",
			"tags":           "Optional ACL tags applied to devices that join with this key (e.g. [\"tag:ci\"])",
			"expiry_seconds": "Key lifetime in seconds (default 90 days)",
			"description":    "Short purpose label (max 50 chars)",
		},
	},
	{
		Name:        mcp.ToolName("tailscale_delete_key"),
		Description: "Revoke (delete) a Tailscale auth key, API access token, or OAuth client by key_id. Devices already joined stay connected. Use after list_keys.",
		Parameters:  map[string]string{"key_id": "Key ID from list_keys"},
		Required:    []string{"key_id"},
	},

	// Policy file
	{
		Name:        mcp.ToolName("tailscale_get_policy_file"),
		Description: "Get the Tailscale tailnet policy file (ACLs, access control rules, groups, tagOwners, hosts, SSH rules, grants, tests) as HuJSON text with comments preserved, plus its etag. Start here before editing ACLs; pass the etag to set_policy_file.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("tailscale_validate_policy_file"),
		Description: "Validate a proposed Tailscale ACL policy file (syntax and embedded tests) without saving it, or run ACL access tests against the current policy. Use before set_policy_file. Returns {valid:true} or the failure message and details.",
		Parameters: map[string]string{
			"policy": "Proposed full policy file as HuJSON or JSON text",
			"tests":  "Alternative to policy: array of ACL tests, e.g. [{\"src\":\"user@example.com\",\"accept\":[\"tag:web:443\"],\"deny\":[\"tag:db:5432\"]}]",
		},
	},
	{
		Name:        mcp.ToolName("tailscale_set_policy_file"),
		Description: "Replace the Tailscale tailnet policy file (ACLs, access control rules). Requires the etag from get_policy_file to avoid overwriting concurrent edits. Send the complete edited HuJSON, keeping existing comments. Use after get_policy_file and validate_policy_file.",
		Parameters: map[string]string{
			"policy":   "Complete new policy file as HuJSON or JSON text",
			"if_match": "etag from get_policy_file, or ts-default to replace only an untouched default policy",
		},
		Required: []string{"policy", "if_match"},
	},
	{
		Name:        mcp.ToolName("tailscale_preview_policy_rules"),
		Description: "Preview which Tailscale ACL rules apply to a user (what can this user reach) or to an IP:port destination (who can reach this service), against the current or a proposed policy. Use to answer access questions or check an ACL change before saving.",
		Parameters: map[string]string{
			"type":        "user (preview_for is a user email) or ipport (preview_for is IP:port like 100.64.0.1:22)",
			"preview_for": "User email or IP:port to evaluate",
			"policy":      "Optional proposed policy HuJSON; defaults to the current policy file",
		},
		Required: []string{"type", "preview_for"},
	},

	// DNS
	{
		Name:        mcp.ToolName("tailscale_get_dns"),
		Description: "Get the Tailscale tailnet DNS configuration: global nameservers, split DNS routes, search paths, MagicDNS, and override local DNS preferences. Empty sections are omitted: a missing nameservers, splitDNS, or searchPaths field means none configured, not an error.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("tailscale_set_dns_nameservers"),
		Description: "Replace the Tailscale tailnet global DNS nameservers (resolvers). Removing all nameservers disables MagicDNS. Use after get_dns.",
		Parameters: map[string]string{
			"nameservers": "Full list of nameserver IPs (e.g. [\"1.1.1.1\",\"8.8.8.8\"]); empty list removes all",
		},
		Required: []string{"nameservers"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_dns_preferences"),
		Description: "Enable or disable Tailscale MagicDNS for the tailnet (automatic machine name DNS). Use after get_dns.",
		Parameters:  map[string]string{"magic_dns": "true to enable MagicDNS, false to disable"},
		Required:    []string{"magic_dns"},
	},
	{
		Name:        mcp.ToolName("tailscale_set_dns_search_paths"),
		Description: "Replace the Tailscale tailnet DNS search domains (search paths). Use after get_dns.",
		Parameters: map[string]string{
			"search_paths": "Full list of search domains (e.g. [\"corp.example.com\"]); empty list removes all",
		},
		Required: []string{"search_paths"},
	},
	{
		Name:        mcp.ToolName("tailscale_update_split_dns"),
		Description: "Add, change, or remove Tailscale split DNS routes (send queries for a domain to specific nameservers). Only listed domains change. Use after get_dns.",
		Parameters: map[string]string{
			"split_dns": "Object mapping domain to nameserver IP list, or null to remove the domain (e.g. {\"corp.example.com\":[\"10.0.0.53\"],\"old.example.com\":null})",
		},
		Required: []string{"split_dns"},
	},

	// Users
	{
		Name:        mcp.ToolName("tailscale_list_users"),
		Description: "List Tailscale users (members) in the tailnet with login email, role, status (active, idle, suspended, needs-approval), device count, and last seen. Use to find a user_id or users waiting for approval.",
		Parameters: map[string]string{
			"type": "member (default), shared, or all",
			"role": "Optional role filter: owner, admin, it-admin, network-admin, billing-admin, auditor, member, all",
		},
	},
	{
		Name:        mcp.ToolName("tailscale_get_user"),
		Description: "Get one Tailscale user by user_id with role, status, device count, and last seen. Use after list_users.",
		Parameters:  map[string]string{"user_id": userIDParam},
		Required:    []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_update_user_role"),
		Description: "Change a Tailscale user's role (admin, network-admin, it-admin, billing-admin, auditor, member, owner). Use after list_users.",
		Parameters: map[string]string{
			"user_id": userIDParam,
			"role":    "New role: owner, admin, it-admin, network-admin, billing-admin, auditor, member",
		},
		Required: []string{"user_id", "role"},
	},
	{
		Name:        mcp.ToolName("tailscale_approve_user"),
		Description: "Approve a Tailscale user waiting for user approval (status needs-approval) so they can join the tailnet. Use after list_users.",
		Parameters:  map[string]string{"user_id": userIDParam},
		Required:    []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_suspend_user"),
		Description: "Suspend a Tailscale user, disconnecting all their devices until restored. Use for offboarding or incident response.",
		Parameters:  map[string]string{"user_id": userIDParam},
		Required:    []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_restore_user"),
		Description: "Restore a suspended Tailscale user, reconnecting their devices. Use after suspend_user.",
		Parameters:  map[string]string{"user_id": userIDParam},
		Required:    []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_delete_user"),
		Description: "Delete (remove) a Tailscale user and their devices from the tailnet. Destructive and irreversible; prefer suspend_user when unsure.",
		Parameters:  map[string]string{"user_id": userIDParam},
		Required:    []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_list_user_invites"),
		Description: "List pending Tailscale user invites to join the tailnet, with email, role, and invite URL.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("tailscale_create_user_invite"),
		Description: "Invite a new user to join the Tailscale tailnet, optionally emailing the invite and assigning a role. Returns the invite URL. Requires a user-owned API access token (not an OAuth client).",
		Parameters: map[string]string{
			"email": "Optional email to send the invite to; when omitted only an invite URL is returned",
			"role":  "Optional role: member (default), admin, it-admin, network-admin, billing-admin, auditor",
		},
	},
	{
		Name:        mcp.ToolName("tailscale_delete_user_invite"),
		Description: "Cancel (delete) a pending Tailscale user invite. Use after list_user_invites.",
		Parameters:  map[string]string{"invite_id": "Invite ID from list_user_invites"},
		Required:    []string{"invite_id"},
	},

	// Tailnet settings, webhooks, contacts
	{
		Name:        mcp.ToolName("tailscale_get_tailnet_settings"),
		Description: "Get Tailscale tailnet settings: device approval, user approval, device auto-updates, key expiry duration, network flow logging, regional routing, posture collection, HTTPS certificates, and external ACL management.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("tailscale_update_tailnet_settings"),
		Description: "Update Tailscale tailnet settings (partial update). Only fields provided change. Use after get_tailnet_settings.",
		Parameters: map[string]string{
			"settings": "Object of setting fields to change, e.g. {\"devicesApprovalOn\":true,\"devicesKeyDurationDays\":90,\"usersApprovalOn\":true,\"devicesAutoUpdatesOn\":true,\"networkFlowLoggingOn\":false,\"httpsEnabled\":true}",
		},
		Required: []string{"settings"},
	},
	{
		Name:        mcp.ToolName("tailscale_list_webhooks"),
		Description: "List Tailscale webhook endpoints and the tailnet events they subscribe to (nodeCreated, nodeNeedsApproval, policyUpdate, userCreated, and more). Signing secrets are omitted.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("tailscale_create_webhook"),
		Description: "Create a Tailscale webhook endpoint that receives tailnet event notifications (device created, needs approval, key expiring, policy updated, user events). The signing secret is only returned once in this response.",
		Parameters: map[string]string{
			"endpoint_url":  "HTTPS URL that receives POSTed events",
			"subscriptions": "Event types, e.g. [\"nodeCreated\",\"nodeNeedsApproval\",\"nodeKeyExpiringInOneDay\",\"policyUpdate\",\"userNeedsApproval\"]",
			"provider_type": "Optional formatting for chat: slack, mattermost, googlechat, discord (omit for generic JSON)",
		},
		Required: []string{"endpoint_url", "subscriptions"},
	},
	{
		Name:        mcp.ToolName("tailscale_delete_webhook"),
		Description: "Delete a Tailscale webhook endpoint. Use after list_webhooks.",
		Parameters:  map[string]string{"webhook_id": "Webhook endpointId from list_webhooks"},
		Required:    []string{"webhook_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_test_webhook"),
		Description: "Send a test event to a Tailscale webhook endpoint to verify delivery. Use after create_webhook or list_webhooks.",
		Parameters:  map[string]string{"webhook_id": "Webhook endpointId from list_webhooks"},
		Required:    []string{"webhook_id"},
	},
	{
		Name:        mcp.ToolName("tailscale_get_contacts"),
		Description: "Get Tailscale tailnet contact preferences (account, support, and security notification emails).",
		Parameters:  map[string]string{},
	},
}
