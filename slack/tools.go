package slack

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Token Management ---
	{
		Name:        "slack_token_status",
		Description: "Check token health, age, auto-refresh status, and source.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "slack_refresh_tokens",
		Description: "Force refresh tokens by extracting from Chrome (requires Slack tab open in Chrome, macOS only).",
		Parameters:  map[string]string{},
	},

	// --- Conversations ---
	{
		Name:        "slack_list_conversations",
		Description: "List channels and DMs in the workspace. Filter by type (public_channel, private_channel, im, mpim).",
		Parameters: map[string]string{
			"types":            "Comma-separated types: public_channel, private_channel, im, mpim (default: public_channel,private_channel)",
			"limit":            "Max results per page (default 100, max 1000)",
			"cursor":           "Pagination cursor from previous response",
			"exclude_archived": "Exclude archived channels (default true)",
		},
	},
	{
		Name:        "slack_get_conversation_info",
		Description: "Get detailed information about a specific channel or DM.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID",
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        "slack_conversations_history",
		Description: "Get messages from a channel or DM. Returns messages in reverse chronological order.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID",
			"limit":      "Number of messages to fetch (default 50, max 100)",
			"oldest":     "Unix timestamp — get messages after this time",
			"latest":     "Unix timestamp — get messages before this time",
			"cursor":     "Pagination cursor from previous response",
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        "slack_get_thread",
		Description: "Get all replies in a message thread.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID",
			"thread_ts":  "Thread parent message timestamp",
		},
		Required: []string{"channel_id", "thread_ts"},
	},
	{
		Name:        "slack_create_conversation",
		Description: "Create a new channel.",
		Parameters: map[string]string{
			"name":       "Channel name (lowercase, no spaces, max 80 chars)",
			"is_private": "Create as private channel (default false)",
		},
		Required: []string{"name"},
	},
	{
		Name:        "slack_archive_conversation",
		Description: "Archive a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID to archive",
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        "slack_invite_to_conversation",
		Description: "Invite users to a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"user_ids":   "Comma-separated user IDs to invite",
		},
		Required: []string{"channel_id", "user_ids"},
	},
	{
		Name:        "slack_kick_from_conversation",
		Description: "Remove a user from a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"user_id":    "User ID to remove",
		},
		Required: []string{"channel_id", "user_id"},
	},
	{
		Name:        "slack_set_conversation_topic",
		Description: "Set the topic of a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"topic":      "New topic text",
		},
		Required: []string{"channel_id", "topic"},
	},
	{
		Name:        "slack_set_conversation_purpose",
		Description: "Set the purpose of a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"purpose":    "New purpose text",
		},
		Required: []string{"channel_id", "purpose"},
	},
	{
		Name:        "slack_join_conversation",
		Description: "Join a public channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID to join",
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        "slack_leave_conversation",
		Description: "Leave a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID to leave",
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        "slack_rename_conversation",
		Description: "Rename a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"name":       "New channel name",
		},
		Required: []string{"channel_id", "name"},
	},

	// --- Messages ---
	{
		Name:        "slack_send_message",
		Description: "Send a message to a channel or DM. Supports Slack mrkdwn formatting and threads.",
		Parameters: map[string]string{
			"channel_id": "Channel or DM ID to send to",
			"text":       "Message text (supports Slack mrkdwn)",
			"thread_ts":  "Thread timestamp to reply in a thread (optional)",
		},
		Required: []string{"channel_id", "text"},
	},
	{
		Name:        "slack_update_message",
		Description: "Update an existing message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID where the message is",
			"ts":         "Timestamp of the message to update",
			"text":       "New message text",
		},
		Required: []string{"channel_id", "ts", "text"},
	},
	{
		Name:        "slack_delete_message",
		Description: "Delete a message from a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Timestamp of the message to delete",
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        "slack_search_messages",
		Description: "Search messages across the Slack workspace. Supports Slack search syntax (from:@user, in:#channel, has:link, etc.).",
		Parameters: map[string]string{
			"query": "Search query (supports Slack search modifiers)",
			"count": "Number of results (default 20, max 100)",
		},
		Required: []string{"query"},
	},
	{
		Name:        "slack_add_reaction",
		Description: "Add an emoji reaction to a message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp",
			"emoji":      "Emoji name without colons (e.g., thumbsup)",
		},
		Required: []string{"channel_id", "ts", "emoji"},
	},
	{
		Name:        "slack_remove_reaction",
		Description: "Remove an emoji reaction from a message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp",
			"emoji":      "Emoji name without colons",
		},
		Required: []string{"channel_id", "ts", "emoji"},
	},
	{
		Name:        "slack_get_reactions",
		Description: "Get all reactions on a message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp",
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        "slack_add_pin",
		Description: "Pin a message in a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp to pin",
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        "slack_remove_pin",
		Description: "Remove a pinned message.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"ts":         "Message timestamp to unpin",
		},
		Required: []string{"channel_id", "ts"},
	},
	{
		Name:        "slack_list_pins",
		Description: "List all pinned items in a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        "slack_schedule_message",
		Description: "Schedule a message to be sent at a specific time.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"text":       "Message text",
			"post_at":    "Unix timestamp for when to send the message",
			"thread_ts":  "Thread timestamp to reply in a thread (optional)",
		},
		Required: []string{"channel_id", "text", "post_at"},
	},

	// --- Users ---
	{
		Name:        "slack_list_users",
		Description: "List all users in the workspace. Supports pagination for large workspaces.",
		Parameters: map[string]string{
			"limit":  "Max users per page (default 200, max 1000)",
			"cursor": "Pagination cursor",
		},
	},
	{
		Name:        "slack_get_user_info",
		Description: "Get detailed information about a user including profile, status, timezone, and admin status.",
		Parameters: map[string]string{
			"user_id": "Slack user ID",
		},
		Required: []string{"user_id"},
	},
	{
		Name:        "slack_get_user_presence",
		Description: "Get a user's current presence status (active/away).",
		Parameters: map[string]string{
			"user_id": "Slack user ID",
		},
		Required: []string{"user_id"},
	},
	{
		Name:        "slack_list_user_groups",
		Description: "List all user groups (handles) in the workspace.",
		Parameters: map[string]string{
			"include_users":    "Include list of member user IDs (default false)",
			"include_disabled": "Include disabled user groups (default false)",
		},
	},
	{
		Name:        "slack_get_user_group",
		Description: "Get members of a specific user group.",
		Parameters: map[string]string{
			"usergroup_id": "User group ID",
		},
		Required: []string{"usergroup_id"},
	},

	// --- Extras ---
	{
		Name:        "slack_auth_test",
		Description: "Test authentication and get current user/workspace info.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "slack_team_info",
		Description: "Get information about the workspace/team.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "slack_upload_file",
		Description: "Upload a text file or snippet to a channel.",
		Parameters: map[string]string{
			"channels":         "Comma-separated channel IDs to share the file in",
			"content":          "Text content of the file",
			"filename":         "Filename (e.g., report.txt)",
			"title":            "Title for the file",
			"filetype":         "File type identifier (e.g., text, python, javascript)",
			"initial_comment":  "Message to include with the file",
			"thread_ts":        "Thread timestamp to upload into a thread (optional)",
		},
		Required: []string{"channels", "content", "filename"},
	},
	{
		Name:        "slack_list_files",
		Description: "List files shared in the workspace. Filter by channel, user, or type.",
		Parameters: map[string]string{
			"channel_id": "Filter by channel ID (optional)",
			"user_id":    "Filter by user ID (optional)",
			"types":      "Filter by file type: spaces, snippets, images, gdocs, zips, pdfs (optional)",
			"count":      "Number of files to return (default 20, max 100)",
		},
	},
	{
		Name:        "slack_delete_file",
		Description: "Delete a file.",
		Parameters: map[string]string{
			"file_id": "File ID to delete",
		},
		Required: []string{"file_id"},
	},
	{
		Name:        "slack_list_emoji",
		Description: "List all custom emoji in the workspace.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "slack_set_status",
		Description: "Set the authenticated user's status.",
		Parameters: map[string]string{
			"status_text":       "Status text",
			"status_emoji":      "Status emoji (e.g., :house_with_garden:)",
			"status_expiration": "Unix timestamp when status expires (0 for no expiration)",
		},
		Required: []string{"status_text"},
	},
	{
		Name:        "slack_list_bookmarks",
		Description: "List bookmarks in a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
		},
		Required: []string{"channel_id"},
	},
	{
		Name:        "slack_add_bookmark",
		Description: "Add a bookmark to a channel.",
		Parameters: map[string]string{
			"channel_id": "Channel ID",
			"title":      "Bookmark title",
			"link":       "URL to bookmark",
			"emoji":      "Emoji for the bookmark (optional)",
		},
		Required: []string{"channel_id", "title", "link"},
	},
	{
		Name:        "slack_remove_bookmark",
		Description: "Remove a bookmark from a channel.",
		Parameters: map[string]string{
			"channel_id":  "Channel ID",
			"bookmark_id": "Bookmark ID to remove",
		},
		Required: []string{"channel_id", "bookmark_id"},
	},
	{
		Name:        "slack_add_reminder",
		Description: "Create a reminder. Time can be natural language (e.g., 'in 15 minutes', 'tomorrow at 9am').",
		Parameters: map[string]string{
			"text": "Reminder text",
			"time": "When to remind (natural language or Unix timestamp)",
			"user": "User ID to remind (defaults to authenticated user)",
		},
		Required: []string{"text", "time"},
	},
	{
		Name:        "slack_list_reminders",
		Description: "List all reminders for the authenticated user.",
		Parameters:  map[string]string{},
	},
	{
		Name:        "slack_delete_reminder",
		Description: "Delete a reminder.",
		Parameters: map[string]string{
			"reminder_id": "Reminder ID to delete",
		},
		Required: []string{"reminder_id"},
	},
}
