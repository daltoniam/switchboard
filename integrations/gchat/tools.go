package gchat

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Space resource ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("gchat_list_spaces"), Description: "List Google Chat spaces the authenticated user belongs to. Start here for chat, conversations, messages, group chats, rooms, direct messages, DMs, threads, channels — to discover the space IDs needed by every other gchat tool. Returns spaces (full resource names like 'spaces/AAQAtuk0o-A'), their type (SPACE / GROUP_CHAT / DIRECT_MESSAGE), and display name (for named rooms).",
		Parameters: map[string]string{
			"page_size":  "Optional max spaces per page (1-1000, default 100)",
			"page_token": "Optional pagination token from a previous response's nextPageToken",
			"filter":     "Optional filter expression (e.g. 'space_type = \"SPACE\"' for named rooms only, 'space_type = \"DIRECT_MESSAGE\"' for DMs)",
		},
		Required: []string{},
	},
	{
		Name: mcp.ToolName("gchat_get_space"), Description: "Retrieve a single Google Chat space by resource name. Returns the space's display name, type, threading state, and history state.",
		Parameters: map[string]string{
			"space_id": "The space ID — accepts either bare ID ('AAQAtuk0o-A') or full resource name ('spaces/AAQAtuk0o-A')",
		},
		Required: []string{"space_id"},
	},

	// ── Message resource ────────────────────────────────────────────
	{
		Name: mcp.ToolName("gchat_list_messages"), Description: "List messages in a Google Chat space (rooms, group chats, DMs). Returns messages with sender, text, createTime, lastUpdateTime, and thread info. Use filter to narrow by createTime range; use order_by='createTime desc' to get newest-first.",
		Parameters: map[string]string{
			"space_id":     "The space ID (bare or 'spaces/X' form)",
			"page_size":    "Optional max messages per page (1-1000, default 25)",
			"page_token":   "Optional pagination token from a previous response's nextPageToken",
			"filter":       "Optional filter expression (e.g. 'createTime > \"2024-01-01T00:00:00Z\"', 'thread.name = \"spaces/X/threads/Y\"')",
			"order_by":     "Optional ordering — 'createTime' (oldest first, default) or 'createTime desc' (newest first)",
			"show_deleted": "Optional boolean — include deleted messages (default false)",
		},
		Required: []string{"space_id"},
	},
	{
		Name: mcp.ToolName("gchat_get_message"), Description: "Retrieve a single Google Chat message by space + message ID. Returns the message text, sender, createTime, lastUpdateTime, thread, attachments, and any cardsV2 payload.",
		Parameters: map[string]string{
			"space_id":   "The space ID (bare or 'spaces/X' form)",
			"message_id": "The message ID (the segment after 'messages/' in the message name — typically contains dots, e.g. 'UMOmwAAAAAE.UMOmwAAAAAE')",
		},
		Required: []string{"space_id", "message_id"},
	},
	{
		Name: mcp.ToolName("gchat_create_message"), Description: "Send a new message to a Google Chat space. Returns the created message including its resource name. Pass text for a plain-text message; pass cards_v2 (raw JSON) for a card message. Use thread_key with message_reply_option to reply in an existing thread.",
		Parameters: map[string]string{
			"space_id":             "The space ID (bare or 'spaces/X' form) to send the message to",
			"text":                 "Message text (plain text; supports basic formatting like *bold*, _italic_, `code`, and <https://link|label> annotations)",
			"cards_v2":             "Optional raw JSON for cardsV2 payload (array of card objects per the Chat API spec). Use instead of or alongside text.",
			"thread_key":           "Optional client-supplied thread key — messages sharing the same key go in one thread (requires message_reply_option)",
			"message_reply_option": "Optional reply behavior: 'REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD' (reply in thread; new thread if no match), 'REPLY_MESSAGE_OR_FAIL' (reply or fail)",
			"message_id":           "Optional custom message ID (alphanumeric, max 56 chars, must start with 'client-')",
		},
		Required: []string{"space_id"},
	},
	{
		Name: mcp.ToolName("gchat_update_message"), Description: "Update an existing Google Chat message (edit its text or replace its cardsV2 content). Uses PATCH semantics with an auto-generated updateMask — only the fields you pass are changed. Only messages sent by the authenticated user / app can be updated.",
		Parameters: map[string]string{
			"space_id":   "The space ID (bare or 'spaces/X' form)",
			"message_id": "The message ID",
			"text":       "Optional new message text",
			"cards_v2":   "Optional new cardsV2 payload (raw JSON array)",
		},
		Required: []string{"space_id", "message_id"},
	},
	{
		Name: mcp.ToolName("gchat_delete_message"), Description: "Delete a Google Chat message. Only messages sent by the authenticated user / app can be deleted. Set force=true to also delete any threaded replies when deleting the thread head.",
		Parameters: map[string]string{
			"space_id":   "The space ID (bare or 'spaces/X' form)",
			"message_id": "The message ID to delete",
			"force":      "Optional boolean — when deleting the head of a thread, also delete the threaded replies (default false)",
		},
		Required: []string{"space_id", "message_id"},
	},

	// ── Membership resource ─────────────────────────────────────────
	{
		Name: mcp.ToolName("gchat_list_members"), Description: "List members (human users and chat apps) of a Google Chat space. Returns each member's name (resource id), role (ROLE_MEMBER / ROLE_MANAGER), state (JOINED / INVITED / NOT_A_MEMBER), and member type (HUMAN / BOT).",
		Parameters: map[string]string{
			"space_id":     "The space ID (bare or 'spaces/X' form)",
			"page_size":    "Optional max members per page (1-1000, default 100)",
			"page_token":   "Optional pagination token from a previous response's nextPageToken",
			"show_invited": "Optional boolean — include invited members (default false; only JOINED members otherwise)",
			"filter":       "Optional filter (e.g. 'member.type = \"HUMAN\"', 'role = \"ROLE_MANAGER\"')",
		},
		Required: []string{"space_id"},
	},
}
