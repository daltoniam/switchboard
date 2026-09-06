package intercom

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("intercom_search_conversations"), Description: "Search Intercom inbox conversations, customer chats, messenger threads, and support messages. Start here for customer support, helpdesk triage, open conversations, and unread inbox work.",
		Parameters: map[string]string{
			"query":          `Intercom search query JSON object (e.g. {"field":"state","operator":"=","value":"open"}). Overrides state when set.`,
			"state":          "Conversation state filter: open, closed, or snoozed. Used when query is omitted.",
			"per_page":       "Results per page (default 20, max 150)",
			"starting_after": "Pagination cursor from pages.next.starting_after",
			"sort_field":     "Sort field (e.g. updated_at, created_at)",
			"sort_order":     "Sort order: ascending or descending",
		},
	},
	{
		Name: mcp.ToolName("intercom_list_conversations"), Description: "List Intercom inbox conversations in reverse chronological order. Prefer search_conversations when filtering by state, assignee, or contact.",
		Parameters: map[string]string{
			"per_page":       "Results per page (default 20, max 150)",
			"starting_after": "Pagination cursor from pages.next.starting_after",
		},
	},
	{
		Name: mcp.ToolName("intercom_get_conversation"), Description: "Get an Intercom conversation including message parts, customer replies, and assignment. Use after search_conversations or list_conversations.",
		Parameters: map[string]string{
			"conversation_id": "Intercom conversation ID",
			"display_as":      "Set to plaintext to strip HTML from message bodies (default plaintext)",
		},
		Required: []string{"conversation_id"},
	},
	{
		Name: mcp.ToolName("intercom_reply_conversation"), Description: "Reply to an Intercom conversation as an admin comment or add an internal note. Use after get_conversation.",
		Parameters: map[string]string{
			"conversation_id":  "Intercom conversation ID",
			"body":             "Reply or note HTML/text body",
			"admin_id":         "Admin ID sending the reply (required for admin replies and notes)",
			"message_type":     "comment (customer-visible reply, default) or note (internal note)",
			"type":             "admin (default) or user",
			"intercom_user_id": "Contact ID when type is user",
			"attachment_urls":  "Optional comma-separated public attachment URLs",
		},
		Required: []string{"conversation_id", "body"},
	},
	{
		Name: mcp.ToolName("intercom_close_conversation"), Description: "Close an Intercom conversation after it is resolved. Use after get_conversation or reply_conversation.",
		Parameters: map[string]string{
			"conversation_id": "Intercom conversation ID",
			"admin_id":        "Admin ID performing the close",
			"body":            "Optional closing message",
		},
		Required: []string{"conversation_id", "admin_id"},
	},
	{
		Name: mcp.ToolName("intercom_reopen_conversation"), Description: "Reopen a closed Intercom conversation. Use after get_conversation.",
		Parameters: map[string]string{
			"conversation_id": "Intercom conversation ID",
			"admin_id":        "Admin ID performing the reopen",
			"body":            "Optional reopen message",
		},
		Required: []string{"conversation_id", "admin_id"},
	},
	{
		Name: mcp.ToolName("intercom_snooze_conversation"), Description: "Snooze an Intercom conversation until a unix timestamp. Use after get_conversation.",
		Parameters: map[string]string{
			"conversation_id": "Intercom conversation ID",
			"admin_id":        "Admin ID performing the snooze",
			"snoozed_until":   "Unix timestamp when the conversation should reopen",
		},
		Required: []string{"conversation_id", "admin_id", "snoozed_until"},
	},
	{
		Name: mcp.ToolName("intercom_assign_conversation"), Description: "Assign an Intercom conversation to an admin teammate or team inbox. Use after list_admins or list_teams.",
		Parameters: map[string]string{
			"conversation_id": "Intercom conversation ID",
			"admin_id":        "Admin ID performing the assignment",
			"assignee_id":     "Admin or team ID to assign the conversation to",
			"body":            "Optional assignment note",
		},
		Required: []string{"conversation_id", "admin_id", "assignee_id"},
	},
	{
		Name: mcp.ToolName("intercom_tag_conversation"), Description: "Add a tag to an Intercom conversation for routing and reporting. Use after list_tags.",
		Parameters: map[string]string{
			"conversation_id": "Intercom conversation ID",
			"tag_id":          "Tag ID from list_tags",
			"admin_id":        "Admin ID applying the tag",
		},
		Required: []string{"conversation_id", "tag_id", "admin_id"},
	},
	{
		Name: mcp.ToolName("intercom_search_contacts"), Description: "Search Intercom contacts, leads, and users by email or a search query. Use to find customers before opening conversations.",
		Parameters: map[string]string{
			"email":          "Contact email address (builds an email equality query when query is omitted)",
			"query":          `Intercom search query JSON object. Overrides email when set.`,
			"per_page":       "Results per page (default 20, max 150)",
			"starting_after": "Pagination cursor from pages.next.starting_after",
		},
	},
	{
		Name: mcp.ToolName("intercom_list_contacts"), Description: "List Intercom contacts, leads, and users. Prefer search_contacts when looking up a specific email.",
		Parameters: map[string]string{
			"per_page":       "Results per page (default 20, max 150)",
			"starting_after": "Pagination cursor from pages.next.starting_after",
		},
	},
	{
		Name: mcp.ToolName("intercom_get_contact"), Description: "Get an Intercom contact, lead, or user by ID including email, name, and custom attributes. Use after search_contacts.",
		Parameters: map[string]string{"contact_id": "Intercom contact ID"},
		Required:   []string{"contact_id"},
	},
	{
		Name: mcp.ToolName("intercom_create_contact"), Description: "Create an Intercom contact, lead, or user. Provide email and/or external_id.",
		Parameters: map[string]string{
			"email":             "Contact email address",
			"external_id":       "Your system's user ID",
			"name":              "Contact name",
			"phone":             "Contact phone number",
			"role":              "user or lead",
			"custom_attributes": "JSON object of custom attributes",
		},
	},
	{
		Name: mcp.ToolName("intercom_update_contact"), Description: "Update an Intercom contact's name, email, phone, or custom attributes. Use after get_contact.",
		Parameters: map[string]string{
			"contact_id":        "Intercom contact ID",
			"email":             "Contact email address",
			"external_id":       "Your system's user ID",
			"name":              "Contact name",
			"phone":             "Contact phone number",
			"role":              "user or lead",
			"custom_attributes": "JSON object of custom attributes",
		},
		Required: []string{"contact_id"},
	},
	{
		Name: mcp.ToolName("intercom_list_companies"), Description: "List Intercom companies and accounts linked to contacts",
		Parameters: map[string]string{
			"name":           "Optional company name filter",
			"company_id":     "Optional external company_id filter",
			"per_page":       "Results per page (default 20)",
			"page":           "Page number (companies use page pagination)",
			"starting_after": "Pagination cursor when provided by a previous response",
		},
	},
	{
		Name: mcp.ToolName("intercom_get_company"), Description: "Get an Intercom company by Intercom ID. Use after list_companies.",
		Parameters: map[string]string{"id": "Intercom company ID (not the external company_id)"},
		Required:   []string{"id"},
	},
	{
		Name: mcp.ToolName("intercom_search_tickets"), Description: "Search Intercom tickets and helpdesk ticket requests by state or query. Use for ticket tracking alongside inbox conversations.",
		Parameters: map[string]string{
			"query":          `Intercom search query JSON object. Overrides state when set.`,
			"state":          "Ticket state filter (e.g. submitted, in_progress, waiting_on_customer, resolved). Used when query is omitted.",
			"per_page":       "Results per page (default 20, max 150)",
			"starting_after": "Pagination cursor from pages.next.starting_after",
		},
	},
	{
		Name: mcp.ToolName("intercom_get_ticket"), Description: "Get an Intercom ticket including attributes, contacts, and assignment. Use after search_tickets.",
		Parameters: map[string]string{"ticket_id": "Intercom ticket ID"},
		Required:   []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("intercom_create_ticket"), Description: "Create an Intercom ticket for a contact. Requires a ticket type from the workspace.",
		Parameters: map[string]string{
			"ticket_type_id":    "Ticket type ID",
			"contact_id":        "Intercom contact ID to attach (used when contacts is omitted)",
			"contacts":          `JSON array of contacts (e.g. [{"id":"abc"}])`,
			"title":             "Ticket title",
			"description":       "Ticket description",
			"admin_assignee_id": "Admin ID to assign",
			"team_assignee_id":  "Team ID to assign",
		},
		Required: []string{"ticket_type_id"},
	},
	{
		Name: mcp.ToolName("intercom_update_ticket"), Description: "Update an Intercom ticket state or assignment. Use after get_ticket.",
		Parameters: map[string]string{
			"ticket_id":         "Intercom ticket ID",
			"open":              "true to open, false to close",
			"admin_assignee_id": "Admin ID to assign",
			"team_assignee_id":  "Team ID to assign",
			"ticket_attributes": "JSON object of ticket attributes",
		},
		Required: []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("intercom_reply_ticket"), Description: "Reply to an Intercom ticket as an admin comment or internal note. Use after get_ticket.",
		Parameters: map[string]string{
			"ticket_id":        "Intercom ticket ID",
			"body":             "Reply or note body",
			"admin_id":         "Admin ID sending the reply (required for admin replies and notes)",
			"message_type":     "comment (default) or note",
			"type":             "admin (default) or user",
			"intercom_user_id": "Contact ID when type is user",
		},
		Required: []string{"ticket_id", "body"},
	},
	{
		Name: mcp.ToolName("intercom_search_articles"), Description: "Search Intercom help center articles and knowledge base docs by phrase",
		Parameters: map[string]string{
			"phrase":         "Search phrase",
			"per_page":       "Results per page (default 20)",
			"starting_after": "Pagination cursor from pages.next.starting_after",
		},
		Required: []string{"phrase"},
	},
	{
		Name: mcp.ToolName("intercom_list_articles"), Description: "List Intercom help center articles and knowledge base docs",
		Parameters: map[string]string{
			"per_page":       "Results per page (default 20)",
			"starting_after": "Pagination cursor from pages.next.starting_after",
		},
	},
	{
		Name: mcp.ToolName("intercom_get_article"), Description: "Get an Intercom help center article body. Use after search_articles or list_articles.",
		Parameters: map[string]string{"article_id": "Intercom article ID"},
		Required:   []string{"article_id"},
	},
	{
		Name: mcp.ToolName("intercom_list_admins"), Description: "List Intercom admins, teammates, and inbox agents. Use to resolve admin_id for replies and assignment.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("intercom_get_me"), Description: "Get the Intercom admin and workspace for the current access token",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("intercom_list_teams"), Description: "List Intercom inbox teams used for conversation assignment",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("intercom_get_team"), Description: "Get an Intercom team by ID including admin members. Use after list_teams.",
		Parameters: map[string]string{"team_id": "Intercom team ID"},
		Required:   []string{"team_id"},
	},
	{
		Name: mcp.ToolName("intercom_list_tags"), Description: "List Intercom tags used to label conversations and contacts",
		Parameters: map[string]string{},
	},
}
