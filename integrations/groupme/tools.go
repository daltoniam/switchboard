package groupme

import mcp "github.com/daltoniam/switchboard"

const sendWarning = "WRITE: sends a real GroupMe message that every recipient sees immediately and that cannot be unsent through this API. Confirm the exact text and destination with the user before calling."

var tools = []mcp.ToolDefinition{
	// ── Account ───────────────────────────────────────────────────
	{
		Name:        "groupme_get_me",
		Description: "Get the authenticated GroupMe user (your own user ID and name). Use to tell your own messages apart from others in group chats and direct messages.",
		Access:      mcp.AccessRead,
	},

	// ── Groups ────────────────────────────────────────────────────
	{
		Name:        "groupme_list_groups",
		Description: "List GroupMe group chats and conversations you belong to, with the latest message preview and message count. Start here for GroupMe messaging: find a group_id to read or send group messages.",
		Parameters: map[string]string{
			"page":     "Page number (default 1)",
			"per_page": "Groups per page (default 10)",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "groupme_get_group",
		Description: "Get one GroupMe group chat with its members (user_id, nickname, roles) and message count. Use after groupme_list_groups; member user_id values work as recipient_id for direct messages.",
		Parameters: map[string]string{
			"group_id": "Group ID from groupme_list_groups",
		},
		Required: []string{"group_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "groupme_list_group_messages",
		Description: "Read messages in a GroupMe group chat, newest first. Page backwards with before_id (oldest id from the previous page); fetch new messages with since_id or after_id. Use after groupme_list_groups.",
		Parameters: map[string]string{
			"group_id":  "Group ID from groupme_list_groups",
			"before_id": "Return messages older than this message ID",
			"since_id":  "Return the most recent messages newer than this message ID (may skip some if many arrived)",
			"after_id":  "Return messages immediately after this message ID, oldest first",
			"limit":     "Messages to return, 1-100 (default 20)",
		},
		Required: []string{"group_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "groupme_send_group_message",
		Description: "Send (post) a text message to a GroupMe group chat as yourself. " + sendWarning + " Use after groupme_list_groups.",
		Parameters: map[string]string{
			"group_id":    "Group ID from groupme_list_groups",
			"text":        "Message text, up to 1000 characters",
			"source_guid": "Optional client-unique ID; GroupMe drops a repeat with the same ID within a minute. Generated when omitted",
		},
		Required: []string{"group_id", "text"},
		Access:   mcp.AccessWrite,
	},

	// ── Direct messages ───────────────────────────────────────────
	{
		Name:        "groupme_list_chats",
		Description: "List GroupMe direct message (DM) conversations, most recently active first, with the other user and the last message. Start here for one-to-one chats: find other_user.id for direct messages.",
		Parameters: map[string]string{
			"page":     "Page number (default 1)",
			"per_page": "Chats per page (default 20)",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "groupme_list_direct_messages",
		Description: "Read GroupMe direct messages (DMs) with one other user, newest first. Page backwards with before_id; fetch new messages with since_id. Use after groupme_list_chats.",
		Parameters: map[string]string{
			"other_user_id": "The other user's ID (other_user.id from groupme_list_chats)",
			"before_id":     "Return messages older than this message ID",
			"since_id":      "Return messages newer than this message ID",
		},
		Required: []string{"other_user_id"},
		Access:   mcp.AccessRead,
	},
	{
		Name:        "groupme_send_direct_message",
		Description: "Send a GroupMe direct message (DM) to one user as yourself. " + sendWarning + " Use after groupme_list_chats or groupme_get_group to find the recipient's user ID.",
		Parameters: map[string]string{
			"recipient_id": "Recipient's user ID",
			"text":         "Message text, up to 1000 characters",
			"source_guid":  "Optional client-unique ID; GroupMe drops a repeat with the same ID within a minute. Generated when omitted",
		},
		Required: []string{"recipient_id", "text"},
		Access:   mcp.AccessWrite,
	},

	// ── Bots ──────────────────────────────────────────────────────
	{
		Name:        "groupme_list_bots",
		Description: "List GroupMe bots you created, with the bot_id and the group each bot posts to. Use before groupme_post_bot_message.",
		Access:      mcp.AccessRead,
	},
	{
		Name:        "groupme_post_bot_message",
		Description: "Post a message to a GroupMe group as one of your bots (shown under the bot's name, not yours). " + sendWarning + " Use after groupme_list_bots.",
		Parameters: map[string]string{
			"bot_id":      "Bot ID from groupme_list_bots",
			"text":        "Message text",
			"picture_url": "Optional image URL already uploaded to the GroupMe image service (i.groupme.com)",
		},
		Required: []string{"bot_id", "text"},
		Access:   mcp.AccessWrite,
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"groupme_get_me":               getMe,
	"groupme_list_groups":          listGroups,
	"groupme_get_group":            getGroup,
	"groupme_list_group_messages":  listGroupMessages,
	"groupme_send_group_message":   sendGroupMessage,
	"groupme_list_chats":           listChats,
	"groupme_list_direct_messages": listDirectMessages,
	"groupme_send_direct_message":  sendDirectMessage,
	"groupme_list_bots":            listBots,
	"groupme_post_bot_message":     postBotMessage,
}
