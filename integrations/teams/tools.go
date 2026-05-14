package teams

import mcp "github.com/daltoniam/switchboard"

const tenantIDDesc = "Tenant ID (omit to use default tenant). Use teams_list_tenants to see configured tenants."

var tools = []mcp.ToolDefinition{
	// --- Auth + tenant management ---
	{
		Name:        mcp.ToolName("teams_login"),
		Description: "Start here to authenticate. Begins the Microsoft device-code OAuth flow. Returns a user_code + verification URL that the human must visit and approve. Call teams_login_poll afterwards to detect when authorization completes. Reuses any in-progress flow on repeat calls.",
		Parameters: map[string]string{
			"tenant": "Optional tenant hint (\"common\" works for personal + work accounts; pass a tenant ID/domain to restrict). Defaults to common.",
		},
	},
	{
		Name:        mcp.ToolName("teams_login_poll"),
		Description: "Poll the in-progress OAuth flow started by teams_login. Returns status=pending until the user approves, then status=complete with the resolved tenant_id / user identity. Returns status=error on rejection or timeout.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("teams_token_status"),
		Description: "Show token health for every configured tenant: access-token expiry, refresh-token availability, default tenant, identity. Use to diagnose auth issues before opening a support ticket.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("teams_refresh_tokens"),
		Description: "Force a refresh of the access token for the given (or default) tenant. Useful after long idle periods or when teams_token_status shows expiry warnings.",
		Parameters: map[string]string{
			"tenant_id": tenantIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("teams_list_tenants"),
		Description: "List all configured Microsoft 365 tenants for this user, with their tenant IDs and identities. Use to find the tenant_id to pass into other tools when working across multiple organizations.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("teams_remove_tenant"),
		Description: "Forget the stored tokens for a tenant. The user will need to teams_login again to use it.",
		Parameters: map[string]string{
			"tenant_id": "Tenant ID to remove",
		},
		Required: []string{"tenant_id"},
	},
	{
		Name:        mcp.ToolName("teams_set_default"),
		Description: "Set the default tenant used when tools are called without an explicit tenant_id.",
		Parameters: map[string]string{
			"tenant_id": "Tenant ID to make default",
		},
		Required: []string{"tenant_id"},
	},
	{
		Name:        mcp.ToolName("teams_get_me"),
		Description: "Return the authenticated user's Microsoft Graph profile (id, displayName, userPrincipalName, mail, jobTitle). Use to confirm whose account the integration is acting as.",
		Parameters: map[string]string{
			"tenant_id": tenantIDDesc,
		},
	},

	// --- Chats (1:1, group, meeting) ---
	{
		Name:        mcp.ToolName("teams_list_chats"),
		Description: "Start here to discover chats. Lists the signed-in user's 1:1, group, and meeting chats in Microsoft Teams. Returns chat IDs needed by teams_list_chat_messages and teams_send_chat_message.",
		Parameters: map[string]string{
			"top":       "Max chats to return (default 20, max 50)",
			"filter":    "OData $filter expression (e.g. \"chatType eq 'oneOnOne'\")",
			"orderby":   "OData $orderby (e.g. \"lastMessagePreview/createdDateTime desc\")",
			"expand":    "Optional expand clause (e.g. \"members,lastMessagePreview\")",
			"tenant_id": tenantIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("teams_get_chat"),
		Description: "Get metadata for a single chat by ID. Use after teams_list_chats to inspect topic, members, or chat type.",
		Parameters: map[string]string{
			"chat_id":   "Chat ID (e.g. 19:xxx@thread.v2)",
			"expand":    "Optional expand clause (e.g. \"members\")",
			"tenant_id": tenantIDDesc,
		},
		Required: []string{"chat_id"},
	},
	{
		Name:        mcp.ToolName("teams_list_chat_messages"),
		Description: "Start here to read chat messages. Returns messages in a 1:1, group, or meeting chat in reverse chronological order. Requires chat_id from teams_list_chats.",
		Parameters: map[string]string{
			"chat_id":   "Chat ID",
			"top":       "Number of messages to return (default 20, max 50)",
			"orderby":   "OData $orderby (default: createdDateTime desc)",
			"tenant_id": tenantIDDesc,
		},
		Required: []string{"chat_id"},
	},
	{
		Name:        mcp.ToolName("teams_get_chat_message"),
		Description: "Get a single chat message by ID, including full HTML body and attachments.",
		Parameters: map[string]string{
			"chat_id":    "Chat ID",
			"message_id": "Message ID",
			"tenant_id":  tenantIDDesc,
		},
		Required: []string{"chat_id", "message_id"},
	},
	{
		Name:        mcp.ToolName("teams_send_chat_message"),
		Description: "Send a message to an existing chat as the signed-in user. Content is HTML by default; set content_type=text for plain text.",
		Parameters: map[string]string{
			"chat_id":      "Chat ID",
			"content":      "Message body (HTML or plain text)",
			"content_type": "Either \"html\" (default) or \"text\"",
			"subject":      "Optional subject line",
			"tenant_id":    tenantIDDesc,
		},
		Required: []string{"chat_id", "content"},
	},
	{
		Name:        mcp.ToolName("teams_list_chat_members"),
		Description: "List the participants in a chat, with display names and user IDs.",
		Parameters: map[string]string{
			"chat_id":   "Chat ID",
			"tenant_id": tenantIDDesc,
		},
		Required: []string{"chat_id"},
	},

	// --- Teams + channels ---
	{
		Name:        mcp.ToolName("teams_list_joined_teams"),
		Description: "Start here to discover teams. Lists the Microsoft Teams (groups) the signed-in user belongs to. Returns team IDs needed by teams_list_channels and downstream tools.",
		Parameters: map[string]string{
			"tenant_id": tenantIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("teams_list_channels"),
		Description: "List channels in a Team. Requires team_id from teams_list_joined_teams.",
		Parameters: map[string]string{
			"team_id":   "Team (group) ID",
			"filter":    "Optional OData $filter (e.g. \"membershipType eq 'standard'\")",
			"tenant_id": tenantIDDesc,
		},
		Required: []string{"team_id"},
	},
	{
		Name:        mcp.ToolName("teams_get_channel"),
		Description: "Get metadata for a single channel (display name, description, membership type, web URL).",
		Parameters: map[string]string{
			"team_id":    "Team ID",
			"channel_id": "Channel ID",
			"tenant_id":  tenantIDDesc,
		},
		Required: []string{"team_id", "channel_id"},
	},
	{
		Name:        mcp.ToolName("teams_list_channel_messages"),
		Description: "List the top-level messages in a channel. Note: this endpoint requires application or admin-consent permissions (ChannelMessage.Read.All); it may fail with 403 on tenants that did not consent. Use teams_list_chat_messages for chat-based reads.",
		Parameters: map[string]string{
			"team_id":    "Team ID",
			"channel_id": "Channel ID",
			"top":        "Number of messages to return (default 20, max 50)",
			"tenant_id":  tenantIDDesc,
		},
		Required: []string{"team_id", "channel_id"},
	},
	{
		Name:        mcp.ToolName("teams_get_channel_message"),
		Description: "Get a single channel message by ID, including HTML body and attachments. Subject to the same admin-consent requirement as teams_list_channel_messages.",
		Parameters: map[string]string{
			"team_id":    "Team ID",
			"channel_id": "Channel ID",
			"message_id": "Message ID",
			"tenant_id":  tenantIDDesc,
		},
		Required: []string{"team_id", "channel_id", "message_id"},
	},
	{
		Name:        mcp.ToolName("teams_list_message_replies"),
		Description: "List the replies (thread) to a channel message. Subject to the same admin-consent requirement as teams_list_channel_messages.",
		Parameters: map[string]string{
			"team_id":    "Team ID",
			"channel_id": "Channel ID",
			"message_id": "Root message ID",
			"top":        "Number of replies (default 20, max 50)",
			"tenant_id":  tenantIDDesc,
		},
		Required: []string{"team_id", "channel_id", "message_id"},
	},
	{
		Name:        mcp.ToolName("teams_send_channel_message"),
		Description: "Post a new top-level message in a channel as the signed-in user. Uses ChannelMessage.Send which does NOT require admin consent in most tenants.",
		Parameters: map[string]string{
			"team_id":      "Team ID",
			"channel_id":   "Channel ID",
			"content":      "Message body (HTML or plain text)",
			"content_type": "Either \"html\" (default) or \"text\"",
			"subject":      "Optional subject line",
			"tenant_id":    tenantIDDesc,
		},
		Required: []string{"team_id", "channel_id", "content"},
	},
	{
		Name:        mcp.ToolName("teams_reply_to_channel_message"),
		Description: "Post a reply in an existing channel thread as the signed-in user.",
		Parameters: map[string]string{
			"team_id":      "Team ID",
			"channel_id":   "Channel ID",
			"message_id":   "Root message ID being replied to",
			"content":      "Reply body (HTML or plain text)",
			"content_type": "Either \"html\" (default) or \"text\"",
			"tenant_id":    tenantIDDesc,
		},
		Required: []string{"team_id", "channel_id", "message_id", "content"},
	},

	// --- Users + presence ---
	{
		Name:        mcp.ToolName("teams_list_users"),
		Description: "List users in the directory. Supports OData $filter, $select, $top. Returns user IDs and UPNs needed by chat lookups and mentions.",
		Parameters: map[string]string{
			"top":       "Max users (default 20, max 100)",
			"filter":    "OData $filter (e.g. \"startswith(displayName,'Jane')\")",
			"select":    "OData $select comma-separated fields",
			"search":    "Search across displayName + UPN (uses Graph $search with ConsistencyLevel=eventual)",
			"tenant_id": tenantIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("teams_get_user"),
		Description: "Get a single user by id, UPN, or email. Includes job title, mail, office location.",
		Parameters: map[string]string{
			"user":      "User ID, UPN, or email",
			"select":    "Optional OData $select",
			"tenant_id": tenantIDDesc,
		},
		Required: []string{"user"},
	},
	{
		Name:        mcp.ToolName("teams_search_users"),
		Description: "Convenience wrapper over teams_list_users for keyword search across displayName + UPN.",
		Parameters: map[string]string{
			"query":     "Search query",
			"top":       "Max results (default 10, max 25)",
			"tenant_id": tenantIDDesc,
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("teams_get_presence"),
		Description: "Get the current Teams presence (availability + activity) for a user. Useful to check whether someone is Available, Busy, DoNotDisturb, or Away before paging them.",
		Parameters: map[string]string{
			"user":      "User ID or UPN. Defaults to the signed-in user.",
			"tenant_id": tenantIDDesc,
		},
	},
}
