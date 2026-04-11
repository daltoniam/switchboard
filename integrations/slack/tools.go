package slack

import mcp "github.com/daltoniam/switchboard"

const teamIDDesc = "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."

var tools = []mcp.ToolDefinition{
	// --- Token Management ---
	{
		Name:        mcp.ToolName("slack_token_status"),
		Description: "Check token health for all workspaces: type (OAuth vs browser session), age, auto-refresh status, and source.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("slack_refresh_tokens"),
		Description: "Force refresh browser session tokens. Tries cookie-based HTTP refresh first (works on all platforms), then Chrome LevelDB extraction (macOS). OAuth tokens (xoxp-) don't need refresh.",
		Parameters: map[string]string{
			"team_id": teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_list_workspaces"),
		Description: "List all configured Slack workspaces with team IDs and names. Use to find the team_id for a specific workspace.",
		Parameters:  map[string]string{},
	},

	// --- Conversations ---
	{
		Name:        mcp.ToolName("slack_list_conversations"),
		Description: "Start here to discover channels. List channels and DMs in the workspace. Filter by type (public_channel, private_channel, im, mpim). Returns channel IDs needed by most other Slack tools.",
		Parameters: map[string]string{
			"types":            "Comma-separated types: public_channel, private_channel, im, mpim (default: public_channel,private_channel)",
			"limit":            "Max results per page (default 100, max 1000)",
			"cursor":           "Pagination cursor from previous response",
			"exclude_archived": "Exclude archived channels (default true)",
			"team_id":          teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_get_conversation_info"),
		Description: "Get detailed information about a specific channel or DM, including topic, purpose, and member count. Use after list_conversations to inspect a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        mcp.ToolName("slack_conversations_history"),
		Description: "Start here to read channel messages. Returns messages in reverse chronological order. Requires channel ID (C...), not channel name. Use list_conversations to find IDs.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID",
			"limit":      "Number of messages to fetch (default 50, max 100)",
			"oldest":     "Unix timestamp — get messages after this time",
			"latest":     "Unix timestamp — get messages before this time",
			"cursor":     "Pagination cursor from previous response",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        mcp.ToolName("slack_get_thread"),
		Description: "Get all replies in a message thread. Use after conversations_history to read full thread replies. Requires channel ID and thread_ts from conversations_history.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID",
			"thread_ts":  "Thread parent message timestamp",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "thread_ts"},
	},
	{
		Name:        mcp.ToolName("slack_create_conversation"),
		Description: "Create a new channel.",
		Parameters: map[string]string{
			"name":       "Channel name (lowercase, no spaces, max 80 chars)",
			"is_private": "Create as private channel (default false)",
			"team_id":    teamIDDesc,
		},
		Required: []string{"name"},
	},
	{
		Name:        mcp.ToolName("slack_archive_conversation"),
		Description: "Archive a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID to archive",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        mcp.ToolName("slack_invite_to_conversation"),
		Description: "Invite users to a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"user_ids":   "Comma-separated user IDs to invite",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "user_ids"},
	},
	{
		Name:        mcp.ToolName("slack_kick_from_conversation"),
		Description: "Remove a user from a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"user_id":    "User ID to remove",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "user_id"},
	},
	{
		Name:        mcp.ToolName("slack_set_conversation_topic"),
		Description: "Set the topic of a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"topic":      "New topic text",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "topic"},
	},
	{
		Name:        mcp.ToolName("slack_set_conversation_purpose"),
		Description: "Set the purpose of a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"purpose":    "New purpose text",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "purpose"},
	},
	{
		Name:        mcp.ToolName("slack_join_conversation"),
		Description: "Join a public channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID to join",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        mcp.ToolName("slack_leave_conversation"),
		Description: "Leave a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID to leave",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        mcp.ToolName("slack_rename_conversation"),
		Description: "Rename a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"name":       "New channel name",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "name"},
	},

	// --- Messages ---
	{
		Name:        mcp.ToolName("slack_send_message"),
		Description: "Send (post) a message to a channel or DM. Requires channel ID (C...), not channel name. Supports Slack mrkdwn formatting and threads.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID to send to",
			"text":       "Message text (supports Slack mrkdwn)",
			"thread_ts":  "Thread timestamp to reply in a thread (optional)",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "text"},
	},
	{
		Name:        mcp.ToolName("slack_update_message"),
		Description: "Update an existing message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID where the message is",
			"ts":         "Timestamp of the message to update",
			"text":       "New message text",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "ts", "text"},
	},
	{
		Name:        mcp.ToolName("slack_delete_message"),
		Description: "Delete a message from a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Timestamp of the message to delete",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        mcp.ToolName("slack_search_messages"),
		Description: "Start here to find messages. Search across the entire Slack workspace. Supports Slack search syntax: from:@user, in:#channel, has:link, before:2024-01-01, after:2024-01-01.",
		Parameters: map[string]string{
			"query":   "Search query (supports Slack search modifiers)",
			"count":   "Number of results (default 20, max 100)",
			"team_id": teamIDDesc,
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("slack_add_reaction"),
		Description: "Add an emoji reaction to a message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp",
			"emoji":      "Emoji name without colons (e.g., thumbsup)",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "ts", "emoji"},
	},
	{
		Name:        mcp.ToolName("slack_remove_reaction"),
		Description: "Remove an emoji reaction from a message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp",
			"emoji":      "Emoji name without colons",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "ts", "emoji"},
	},
	{
		Name:        mcp.ToolName("slack_get_reactions"),
		Description: "Get all reactions on a message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        mcp.ToolName("slack_add_pin"),
		Description: "Pin a message in a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp to pin",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        mcp.ToolName("slack_remove_pin"),
		Description: "Remove a pinned message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp to unpin",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        mcp.ToolName("slack_list_pins"),
		Description: "List all pinned items in a channel. Use to find important messages and references pinned by the team.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        mcp.ToolName("slack_schedule_message"),
		Description: "Schedule a message to be sent at a specific time.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"text":       "Message text",
			"post_at":    "Unix timestamp for when to send the message",
			"thread_ts":  "Thread timestamp to reply in a thread (optional)",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "text", "post_at"},
	},

	// --- Users ---
	{
		Name:        mcp.ToolName("slack_list_users"),
		Description: "Start here to find users. List all users in the workspace with display names, emails, and IDs. Supports pagination for large workspaces.",
		Parameters: map[string]string{
			"limit":   "Max users per page (default 200, max 1000)",
			"cursor":  "Pagination cursor",
			"team_id": teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_get_user_info"),
		Description: "Get detailed profile for a single user including status, timezone, and admin status. Use after list_users when you need full details for a specific person.",
		Parameters: map[string]string{
			"user_id": "Slack user ID",
			"team_id": teamIDDesc,
		},
		Required: []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("slack_get_user_presence"),
		Description: "Get a user's current presence status (active/away). Use after list_users to check if someone is online.",
		Parameters: map[string]string{
			"user_id": "Slack user ID",
			"team_id": teamIDDesc,
		},
		Required: []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("slack_list_user_groups"),
		Description: "List all user groups (handles like @engineering) in the workspace. Use to find group IDs and membership.",
		Parameters: map[string]string{
			"include_users":    "Include list of member user IDs (default false)",
			"include_disabled": "Include disabled user groups (default false)",
			"team_id":          teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_get_user_group"),
		Description: "Get members of a specific user group.",
		Parameters: map[string]string{
			"usergroup_id": "User group ID",
			"team_id":      teamIDDesc,
		},
		Required: []string{"usergroup_id"},
	},

	// --- Extras ---
	{
		Name:        mcp.ToolName("slack_auth_test"),
		Description: "Test authentication and get current user/workspace info. Use to verify credentials and find your own user ID.",
		Parameters: map[string]string{
			"team_id": teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_team_info"),
		Description: "Get information about the workspace/team.",
		Parameters: map[string]string{
			"team_id": teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_upload_file"),
		Description: "Upload a text file or snippet to a channel.",
		Parameters: map[string]string{
			"channels":        "Comma-separated channel IDs to share the file in",
			"content":         "Text content of the file",
			"filename":        "Filename (e.g., report.txt)",
			"title":           "Title for the file",
			"filetype":        "File type identifier (e.g., text, python, javascript)",
			"initial_comment": "Message to include with the file",
			"thread_ts":       "Thread timestamp to upload into a thread (optional)",
			"team_id":         teamIDDesc,
		},
		Required: []string{"channels", "content", "filename"},
	},
	{
		Name:        mcp.ToolName("slack_list_files"),
		Description: "List files shared in the workspace. Use to find documents, images, and snippets. Filter by channel, user, or type.",
		Parameters: map[string]string{
			"channel_id": "Filter by channel ID (optional)",
			"user_id":    "Filter by user ID (optional)",
			"types":      "Filter by file type: spaces, snippets, images, gdocs, zips, pdfs (optional)",
			"count":      "Number of files to return (default 20, max 100)",
			"team_id":    teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_delete_file"),
		Description: "Delete a file.",
		Parameters: map[string]string{
			"file_id": "File ID to delete",
			"team_id": teamIDDesc,
		},
		Required: []string{"file_id"},
	},
	{
		Name:        mcp.ToolName("slack_list_emoji"),
		Description: "List all custom emoji in the workspace.",
		Parameters: map[string]string{
			"team_id": teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_set_status"),
		Description: "Set the authenticated user's status.",
		Parameters: map[string]string{
			"status_text":       "Status text",
			"status_emoji":      "Status emoji (e.g., :house_with_garden:)",
			"status_expiration": "Unix timestamp when status expires (0 for no expiration)",
			"team_id":           teamIDDesc,
		},
		Required: []string{"status_text"},
	},
	{
		Name:        mcp.ToolName("slack_list_bookmarks"),
		Description: "List bookmarks in a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        mcp.ToolName("slack_add_bookmark"),
		Description: "Add a bookmark to a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"title":      "Bookmark title",
			"link":       "URL to bookmark",
			"emoji":      "Emoji for the bookmark (optional)",
			"team_id":    teamIDDesc,
		},
		Required: []string{"channel_id", "title", "link"},
	},
	{
		Name:        mcp.ToolName("slack_remove_bookmark"),
		Description: "Remove a bookmark from a channel.",
		Parameters: map[string]string{
			"channel_id":  "Channel ID",
			"bookmark_id": "Bookmark ID to remove",
			"team_id":     teamIDDesc,
		},
		Required: []string{"channel_id", "bookmark_id"},
	},
	{
		Name:        mcp.ToolName("slack_add_reminder"),
		Description: "Create a reminder. Time can be natural language (e.g., 'in 15 minutes', 'tomorrow at 9am').",
		Parameters: map[string]string{
			"text":    "Reminder text",
			"time":    "When to remind (natural language or Unix timestamp)",
			"user":    "User ID to remind (defaults to authenticated user)",
			"team_id": teamIDDesc,
		},
		Required: []string{"text", "time"},
	},
	{
		Name:        mcp.ToolName("slack_list_reminders"),
		Description: "List all reminders for the authenticated user.",
		Parameters: map[string]string{
			"team_id": teamIDDesc,
		},
	},
	{
		Name:        mcp.ToolName("slack_delete_reminder"),
		Description: "Delete a reminder.",
		Parameters: map[string]string{
			"reminder_id": "Reminder ID to delete",
			"team_id":     teamIDDesc,
		},
		Required: []string{"reminder_id"},
	},
}
