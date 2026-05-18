package slack

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Token Management ---
	{
		Name:        mcp.ToolName("slack_token_status"),
		Description: "Check token health for all workspaces: type (OAuth vs browser session), age, auto-refresh status, and source.",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("slack_refresh_tokens"),
		Description: "Force refresh browser session tokens. Tries cookie-based HTTP refresh first (works on all platforms), then Chrome LevelDB extraction (macOS). OAuth tokens (xoxp-) don't need refresh.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_list_workspaces"),
		Description: "List all configured Slack workspaces with team IDs and names. Use to find the team_id for a specific workspace.",
		Parameters:  []mcp.Parameter{},
	},

	// --- Conversations ---
	{
		Name:        mcp.ToolName("slack_list_conversations"),
		Description: "Start here to discover channels. List channels and DMs in the workspace. Filter by type (public_channel, private_channel, im, mpim). Returns channel IDs needed by most other Slack tools.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("types"), Description: "Comma-separated types: public_channel, private_channel, im, mpim (default: public_channel,private_channel)"}, {Name: mcp.ParamName("limit"), Description: "Max results per page (default 100, max 1000)"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}, {Name: mcp.ParamName("exclude_archived"), Description: "Exclude archived channels (default true)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_get_conversation_info"),
		Description: "Get detailed information about a specific channel or DM, including topic, purpose, and member count. Use after list_conversations to inspect a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel or DM ID", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_conversations_history"),
		Description: "Start here to read channel messages. Returns messages in reverse chronological order. Requires channel ID (C...), not channel name. Use list_conversations to find IDs.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel or DM ID", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of messages to fetch (default 50, max 100)"}, {Name: mcp.ParamName("oldest"), Description: "Unix timestamp — get messages after this time"}, {Name: mcp.ParamName("latest"), Description: "Unix timestamp — get messages before this time"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_get_thread"),
		Description: "Get all replies in a message thread. Use after conversations_history to read full thread replies. Requires channel ID and thread_ts from conversations_history.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel or DM ID", Required: true}, {Name: mcp.ParamName("thread_ts"), Description: "Thread parent message timestamp", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_create_conversation"),
		Description: "Create a new channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Channel name (lowercase, no spaces, max 80 chars)", Required: true}, {Name: mcp.ParamName("is_private"), Description: "Create as private channel (default false)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_archive_conversation"),
		Description: "Archive a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID to archive", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_invite_to_conversation"),
		Description: "Invite users to a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("user_ids"), Description: "Comma-separated user IDs to invite", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_kick_from_conversation"),
		Description: "Remove a user from a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("user_id"), Description: "User ID to remove", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_set_conversation_topic"),
		Description: "Set the topic of a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("topic"), Description: "New topic text", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_set_conversation_purpose"),
		Description: "Set the purpose of a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("purpose"), Description: "New purpose text", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_join_conversation"),
		Description: "Join a public channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID to join", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_leave_conversation"),
		Description: "Leave a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID to leave", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_rename_conversation"),
		Description: "Rename a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New channel name", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},

	// --- Messages ---
	{
		Name:        mcp.ToolName("slack_send_message"),
		Description: "Send (post) a message to a channel or DM. Requires channel ID (C...), not channel name. Supports Slack mrkdwn formatting and threads.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel or DM ID to send to", Required: true}, {Name: mcp.ParamName("text"), Description: "Message text (supports Slack mrkdwn)", Required: true}, {Name: mcp.ParamName("thread_ts"), Description: "Thread timestamp to reply in a thread (optional)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_update_message"),
		Description: "Update an existing message.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID where the message is", Required: true}, {Name: mcp.ParamName("ts"), Description: "Timestamp of the message to update", Required: true}, {Name: mcp.ParamName("text"), Description: "New message text", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_delete_message"),
		Description: "Delete a message from a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("ts"), Description: "Timestamp of the message to delete", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_search_messages"),
		Description: "Start here to find messages. Search across the entire Slack workspace. Supports Slack search syntax: from:@user, in:#channel, has:link, before:2024-01-01, after:2024-01-01.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query (supports Slack search modifiers)", Required: true}, {Name: mcp.ParamName("count"), Description: "Number of results (default 20, max 100)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_add_reaction"),
		Description: "Add an emoji reaction to a message.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("ts"), Description: "Message timestamp", Required: true}, {Name: mcp.ParamName("emoji"), Description: "Emoji name without colons (e.g., thumbsup)", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_remove_reaction"),
		Description: "Remove an emoji reaction from a message.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("ts"), Description: "Message timestamp", Required: true}, {Name: mcp.ParamName("emoji"), Description: "Emoji name without colons", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_get_reactions"),
		Description: "Get all reactions on a message.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("ts"), Description: "Message timestamp", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_add_pin"),
		Description: "Pin a message in a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("ts"), Description: "Message timestamp to pin", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_remove_pin"),
		Description: "Remove a pinned message.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("ts"), Description: "Message timestamp to unpin", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_list_pins"),
		Description: "List all pinned items in a channel. Use to find important messages and references pinned by the team.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_schedule_message"),
		Description: "Schedule a message to be sent at a specific time.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("text"), Description: "Message text", Required: true}, {Name: mcp.ParamName("post_at"), Description: "Unix timestamp for when to send the message", Required: true}, {Name: mcp.ParamName("thread_ts"), Description: "Thread timestamp to reply in a thread (optional)"},

		// --- Users ---
		{Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},

	{
		Name:        mcp.ToolName("slack_list_users"),
		Description: "Start here to find users. List all users in the workspace with display names, emails, and IDs. Supports pagination for large workspaces.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("limit"), Description: "Max users per page (default 200, max 1000)"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_get_user_info"),
		Description: "Get detailed profile for a single user including status, timezone, and admin status. Use after list_users when you need full details for a specific person.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "Slack user ID", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_get_user_presence"),
		Description: "Get a user's current presence status (active/away). Use after list_users to check if someone is online.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "Slack user ID", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_list_user_groups"),
		Description: "List all user groups (handles like @engineering) in the workspace. Use to find group IDs and membership.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("include_users"), Description: "Include list of member user IDs (default false)"}, {Name: mcp.ParamName("include_disabled"), Description: "Include disabled user groups (default false)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_get_user_group"),
		Description: "Get members of a specific user group.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("usergroup_id"), Description: "User group ID", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},

	// --- Extras ---
	{
		Name:        mcp.ToolName("slack_auth_test"),
		Description: "Test authentication and get current user/workspace info. Use to verify credentials and find your own user ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_team_info"),
		Description: "Get information about the workspace/team.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_upload_file"),
		Description: "Upload a text file or snippet to a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channels"), Description: "Comma-separated channel IDs to share the file in", Required: true}, {Name: mcp.ParamName("content"), Description: "Text content of the file", Required: true}, {Name: mcp.ParamName("filename"), Description: "Filename (e.g., report.txt)", Required: true}, {Name: mcp.ParamName("title"), Description: "Title for the file"}, {Name: mcp.ParamName("filetype"), Description: "File type identifier (e.g., text, python, javascript)"}, {Name: mcp.ParamName("initial_comment"), Description: "Message to include with the file"}, {Name: mcp.ParamName("thread_ts"), Description: "Thread timestamp to upload into a thread (optional)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_list_files"),
		Description: "List files shared in the workspace. Use to find documents, images, and snippets. Filter by channel, user, or type.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Filter by channel ID (optional)"}, {Name: mcp.ParamName("user_id"), Description: "Filter by user ID (optional)"}, {Name: mcp.ParamName("types"), Description: "Filter by file type: spaces, snippets, images, gdocs, zips, pdfs (optional)"}, {Name: mcp.ParamName("count"), Description: "Number of files to return (default 20, max 100)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_delete_file"),
		Description: "Delete a file.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("file_id"), Description: "File ID to delete", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_list_emoji"),
		Description: "List all custom emoji in the workspace.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_set_status"),
		Description: "Set the authenticated user's status.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("status_text"), Description: "Status text", Required: true}, {Name: mcp.ParamName("status_emoji"), Description: "Status emoji (e.g., :house_with_garden:)"}, {Name: mcp.ParamName("status_expiration"), Description: "Unix timestamp when status expires (0 for no expiration)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_list_bookmarks"),
		Description: "List bookmarks in a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_add_bookmark"),
		Description: "Add a bookmark to a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("title"), Description: "Bookmark title", Required: true}, {Name: mcp.ParamName("link"), Description: "URL to bookmark", Required: true}, {Name: mcp.ParamName("emoji"), Description: "Emoji for the bookmark (optional)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_remove_bookmark"),
		Description: "Remove a bookmark from a channel.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("channel_id"), Description: "Channel ID", Required: true}, {Name: mcp.ParamName("bookmark_id"), Description: "Bookmark ID to remove", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_add_reminder"),
		Description: "Create a reminder. Time can be natural language (e.g., 'in 15 minutes', 'tomorrow at 9am').",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("text"), Description: "Reminder text", Required: true}, {Name: mcp.ParamName("time"), Description: "When to remind (natural language or Unix timestamp)", Required: true}, {Name: mcp.ParamName("user"), Description: "User ID to remind (defaults to authenticated user)"}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_list_reminders"),
		Description: "List all reminders for the authenticated user.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
	{
		Name:        mcp.ToolName("slack_delete_reminder"),
		Description: "Delete a reminder.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("reminder_id"), Description: "Reminder ID to delete", Required: true}, {Name: mcp.ParamName("team_id"), Description: "Workspace team ID (omit to use default workspace). Use slack_list_workspaces to see available workspaces."}},
	},
}
