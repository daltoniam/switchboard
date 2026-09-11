package front

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("front_search_conversations"), Description: "Search Front inbox conversations, shared inbox threads, customer emails, and support tickets with Front search syntax. Start here for helpdesk triage, open conversations, unreplied mail, and finding threads by recipient, assignee, inbox, or tag.",
		Parameters: map[string]string{
			"query":      "Front search query (e.g. 'lost shipment', 'is:open is:unassigned', 'to:user@example.com', 'inbox:inb_41w25'). Text searches subject and body; filters use name:value.",
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
		Required: []string{"query"},
	},
	{
		Name: mcp.ToolName("front_list_conversations"), Description: "List Front inbox conversations in reverse chronological order. Prefer search_conversations for filtered queries by recipient, assignee, or tag.",
		Parameters: map[string]string{
			"statuses":   "Comma-separated conversation statuses: assigned, unassigned, archived, trashed",
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
	},
	{
		Name: mcp.ToolName("front_get_conversation"), Description: "Get a Front conversation including subject, status, assignee, recipient, tags, and ticket IDs. Use after search_conversations or list_conversations. For the message thread, use list_conversation_messages.",
		Parameters: map[string]string{"conversation_id": "Front conversation ID (e.g. cnv_55c8c149)"},
		Required:   []string{"conversation_id"},
	},
	{
		Name: mcp.ToolName("front_list_conversation_messages"), Description: "List messages, emails, and replies in a Front conversation timeline. Use after get_conversation when you need the discussion thread.",
		Parameters: map[string]string{
			"conversation_id": "Front conversation ID",
			"limit":           "Results per page (default 25, max 100)",
			"page_token":      "Pagination cursor from _pagination.next",
		},
		Required: []string{"conversation_id"},
	},
	{
		Name: mcp.ToolName("front_list_conversation_comments"), Description: "List internal Front comments and discussion notes on a conversation. Use after get_conversation. These are teammate-only notes, not customer-visible replies.",
		Parameters: map[string]string{
			"conversation_id": "Front conversation ID",
			"limit":           "Results per page (default 25, max 100)",
			"page_token":      "Pagination cursor from _pagination.next",
		},
		Required: []string{"conversation_id"},
	},
	{
		Name: mcp.ToolName("front_add_comment"), Description: "Add an internal Front comment or discussion note to a conversation. Use after get_conversation. Comments are teammate-only; to send a customer-visible reply use reply_conversation.",
		Parameters: map[string]string{
			"conversation_id": "Front conversation ID",
			"body":            "Comment body. Markdown is supported.",
			"author_id":       "Optional teammate ID or alt:email:user@example.com. Defaults to the API token / OAuth client.",
			"is_pinned":       "Whether to pin the comment (true/false)",
		},
		Required: []string{"conversation_id", "body"},
	},
	{
		Name: mcp.ToolName("front_update_conversation"), Description: "Update a Front conversation status, assignee, inbox, or tags. Use after get_conversation. Status values: archived, open, deleted, spam.",
		Parameters: map[string]string{
			"conversation_id": "Front conversation ID",
			"status":          "archived, open, deleted, or spam",
			"status_id":       "Ticketing status ID (sts_...). Only one of status or status_id.",
			"assignee_id":     "Teammate ID to assign. Use empty string to unassign.",
			"inbox_id":        "Inbox ID to move the conversation to",
			"tag_ids":         "Comma-separated tag IDs replacing existing tags",
		},
		Required: []string{"conversation_id"},
	},
	{
		Name: mcp.ToolName("front_assign_conversation"), Description: "Assign a Front conversation to a teammate or unassign it. Use after list_teammates. Preferred over update_conversation when only changing assignee.",
		Parameters: map[string]string{
			"conversation_id": "Front conversation ID",
			"assignee_id":     "Teammate ID to assign. Omit or leave empty to unassign.",
		},
		Required: []string{"conversation_id"},
	},
	{
		Name: mcp.ToolName("front_list_inboxes"), Description: "List Front shared and personal inboxes used to route conversations. Use to resolve inbox IDs for search filters and conversation moves.",
		Parameters: map[string]string{
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
	},
	{
		Name: mcp.ToolName("front_list_teammates"), Description: "List Front teammates, agents, and inbox users. Use to find assignee_id or author_id for assignment, comments, and sending.",
		Parameters: map[string]string{
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
	},
	{
		Name: mcp.ToolName("front_list_tags"), Description: "List Front tags and labels used to classify conversations. Use before search_conversations with tag: or update_conversation tag_ids.",
		Parameters: map[string]string{
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
	},
	{
		Name: mcp.ToolName("front_list_channels"), Description: "List Front channels (email, SMS, chat, custom) used to send messages. Use before create_message or create_draft to get channel_id.",
		Parameters: map[string]string{
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
	},
	{
		Name: mcp.ToolName("front_list_contacts"), Description: "List Front contacts and customer records. Prefer search_contacts when looking up a specific email or handle.",
		Parameters: map[string]string{
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
	},
	{
		Name: mcp.ToolName("front_search_contacts"), Description: "Find a Front contact, customer, or lead by email handle or contact ID. Use before opening conversations with a customer.",
		Parameters: map[string]string{
			"email":      "Contact email handle (uses alt:email alias)",
			"contact_id": "Front contact ID (crd_...). Used when email is omitted.",
		},
	},
	{
		Name: mcp.ToolName("front_get_contact"), Description: "Get a Front contact including name, handles, and linked accounts. Use after list_contacts or search_contacts.",
		Parameters: map[string]string{"contact_id": "Front contact ID or alias (e.g. crd_55c8c149 or alt:email:user@example.com)"},
		Required:   []string{"contact_id"},
	},
	{
		Name: mcp.ToolName("front_list_accounts"), Description: "List Front accounts and customer companies linked to contacts.",
		Parameters: map[string]string{
			"limit":      "Results per page (default 25, max 100)",
			"page_token": "Pagination cursor from _pagination.next",
		},
	},
	{
		Name: mcp.ToolName("front_get_account"), Description: "Get a Front account or company by ID or domain alias. Use after list_accounts.",
		Parameters: map[string]string{"account_id": "Front account ID or alias (e.g. acc_123 or alt:domain:example.com)"},
		Required:   []string{"account_id"},
	},
	{
		Name: mcp.ToolName("front_create_message"), Description: "Send a new Front email or outbound message from a channel, creating a conversation. This is a send action. Use after list_channels to get channel_id. For replies, prefer reply_conversation.",
		Parameters: map[string]string{
			"channel_id":  "Sending channel ID or alt:address:support@example.com",
			"to":          "Comma-separated recipient handles",
			"cc":          "Comma-separated CC handles",
			"bcc":         "Comma-separated BCC handles",
			"subject":     "Email subject",
			"body":        "Message body",
			"author_id":   "Optional teammate ID sending on behalf of",
			"sender_name": "Optional sender display name",
			"archive":     "Archive the conversation after sending (default true)",
			"tag_ids":     "Comma-separated tag IDs to add",
		},
		Required: []string{"channel_id", "body"},
	},
	{
		Name: mcp.ToolName("front_reply_conversation"), Description: "Send a customer-visible reply on a Front conversation. This is a send action. Use after get_conversation. For internal notes use add_comment.",
		Parameters: map[string]string{
			"conversation_id": "Front conversation ID",
			"channel_id":      "Optional sending channel ID. Defaults to the conversation channel.",
			"to":              "Comma-separated recipient handles",
			"cc":              "Comma-separated CC handles",
			"bcc":             "Comma-separated BCC handles",
			"subject":         "Email subject",
			"body":            "Reply body",
			"author_id":       "Optional teammate ID sending on behalf of",
			"archive":         "Archive the conversation after sending (true/false)",
		},
		Required: []string{"conversation_id", "body"},
	},
	{
		Name: mcp.ToolName("front_create_draft"), Description: "Create a Front draft message in a channel (starts a new conversation) without sending. Use after list_channels. Sending the draft is a separate send action.",
		Parameters: map[string]string{
			"channel_id": "Channel ID or alt:address:support@example.com",
			"to":         "Comma-separated recipient handles",
			"cc":         "Comma-separated CC handles",
			"bcc":        "Comma-separated BCC handles",
			"subject":    "Draft subject",
			"body":       "Draft body",
			"author_id":  "Optional teammate ID",
			"mode":       "private (author only, default) or shared",
		},
		Required: []string{"channel_id", "body"},
	},
}
