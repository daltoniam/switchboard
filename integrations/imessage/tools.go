package imessage

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "imessage_list_chats",
		Description: "List recent iMessage, SMS, and RCS text message conversations (chats, threads, DMs, group texts) on this Mac, newest first, " +
			"with participants, contact names, last message preview, and unread count. Start here for reading texts, finding a conversation, or getting a chat_id.",
		Parameters: map[string]string{
			"query":  "Optional filter matching chat name, contact name, phone number, or email. 1:1 chats with a matching person are listed before group chats",
			"limit":  "Max chats to return (default 20, max 100)",
			"offset": "Number of chats to skip for pagination (default 0)",
		},
	},
	{
		Name: "imessage_get_chat_messages",
		Description: "Read the message history (transcript) of one iMessage/SMS conversation in chronological order, including sender, reactions (tapbacks), " +
			"replies, and attachments. Use after imessage_list_chats with chat_id, or pass handle (phone number or email) to read a 1:1 conversation directly.",
		Parameters: map[string]string{
			"chat_id": "Chat ID from imessage_list_chats",
			"handle":  "Phone number or email of the other person; reads all 1:1 chats with them (alternative to chat_id)",
			"limit":   "Max messages to return (default 30, max 200)",
			"before":  "Only messages before this time (RFC3339 or YYYY-MM-DD). Pass next_before from a previous response to page back",
			"since":   "Only messages at or after this time (RFC3339 or YYYY-MM-DD)",
		},
	},
	{
		Name: "imessage_search_messages",
		Description: "Full-text search across iMessage, SMS, and RCS text messages (case-insensitive), newest first. " +
			"Use to find texts mentioning a word, address, code, or link. Optionally scope to one chat_id, a handle, or a date range.",
		Parameters: map[string]string{
			"query":   "Text to search for",
			"chat_id": "Optional chat ID to search within",
			"handle":  "Optional phone number or email; only messages in conversations with this person",
			"since":   "Optional lower bound time (RFC3339 or YYYY-MM-DD)",
			"before":  "Optional upper bound time (RFC3339 or YYYY-MM-DD)",
			"limit":   "Max messages to return (default 20, max 100)",
		},
		Required: []string{"query"},
	},
	{
		Name:        "imessage_list_unread",
		Description: "List unread incoming iMessage and SMS text messages across all conversations, newest first. Use to check new texts or what needs a reply.",
		Parameters: map[string]string{
			"limit": "Max messages to return (default 50, max 200)",
		},
	},
	{
		Name: "imessage_lookup_contact",
		Description: "Look up people in macOS Contacts (address book) by name, phone number, or email to find the handle to text or message. " +
			"Use before imessage_send_message or imessage_get_chat_messages when only a person's name is known.",
		Parameters: map[string]string{
			"query": "Name, partial phone number, or email to search for",
			"limit": "Max contacts to return (default 10, max 50)",
		},
		Required: []string{"query"},
	},
	{
		Name: "imessage_send_message",
		Description: "Send a text message via iMessage (or SMS) through the Messages app on this Mac. Provide chat_id (from imessage_list_chats, works for group chats) " +
			"or to (phone number or email). Requires allow_send enabled in the integration settings; recipients may be restricted by send_allowlist.",
		Parameters: map[string]string{
			"text":    "Message text to send",
			"chat_id": "Existing chat ID to reply in (preferred; supports group chats)",
			"to":      "Phone number or email to text when no chat_id is given",
			"service": "For 'to' without an existing chat: 'imessage' (default) or 'sms'",
		},
		Required: []string{"text"},
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"imessage_list_chats":        listChats,
	"imessage_get_chat_messages": getChatMessages,
	"imessage_search_messages":   searchMessages,
	"imessage_list_unread":       listUnread,
	"imessage_lookup_contact":    lookupContact,
	"imessage_send_message":      sendMessage,
}
