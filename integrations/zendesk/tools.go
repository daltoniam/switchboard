package zendesk

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("zendesk_search_tickets"), Description: "Search Zendesk support tickets, helpdesk issues, and customer requests with Zendesk search syntax. Start here for ticket triage, open bugs, unresolved customer problems, and finding tickets by requester, assignee, status, or tag.",
		Parameters: map[string]string{
			"query":      "Zendesk search query (e.g. 'status:open assignee:me', 'requester:user@example.com', or free text). type:ticket is added automatically.",
			"sort_by":    "Sort field: updated_at, created_at, priority, status, ticket_type (default updated_at)",
			"sort_order": "asc or desc (default desc)",
			"page":       "Offset page number (default 1)",
			"per_page":   "Results per page (default 25, max 100)",
		},
		Required: []string{"query"},
	},
	{
		Name: mcp.ToolName("zendesk_list_tickets"), Description: "List Zendesk support tickets with cursor pagination. Prefer search_tickets for filtered queries. Use after search when you need recent tickets without a query.",
		Parameters: map[string]string{
			"page_size":   "Page size (default 25, max 100)",
			"page_after":  "Cursor for the next page (meta.after_cursor)",
			"page_before": "Cursor for the previous page (meta.before_cursor)",
			"sort_by":     "Sort: id, created_at, updated_at, status, subject",
			"sort_order":  "asc or desc",
		},
	},
	{
		Name: mcp.ToolName("zendesk_get_ticket"), Description: "Get a Zendesk support ticket by ID including subject, status, requester, assignee, tags, and description. Use after search_tickets or list_tickets.",
		Parameters: map[string]string{
			"ticket_id": "Ticket ID",
			"include":   "Optional comma-separated sideloads (users,groups,organizations,comment_count)",
		},
		Required: []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("zendesk_create_ticket"), Description: "Create a Zendesk support ticket, helpdesk issue, or customer request. Use after list_users to find requester_id or assignee_id.",
		Parameters: map[string]string{
			"subject":         "Ticket subject / title",
			"comment":         "Initial comment body (required by Zendesk)",
			"public":          "Whether the initial comment is public (default true)",
			"status":          "new, open, pending, hold, solved, closed",
			"priority":        "low, normal, high, urgent",
			"type":            "problem, incident, question, task",
			"requester_id":    "Requester user ID",
			"requester_email": "Requester email when creating a new end user",
			"requester_name":  "Requester name when creating a new end user",
			"assignee_id":     "Assignee agent user ID",
			"group_id":        "Group ID",
			"organization_id": "Organization ID",
			"tags":            "Comma-separated tags",
			"custom_fields":   `JSON array of {id, value} custom field objects`,
		},
		Required: []string{"subject", "comment"},
	},
	{
		Name: mcp.ToolName("zendesk_update_ticket"), Description: "Update a Zendesk ticket (status, priority, assignee, tags, subject). Use after get_ticket. To add a comment, prefer add_ticket_comment.",
		Parameters: map[string]string{
			"ticket_id":       "Ticket ID",
			"subject":         "New subject",
			"status":          "new, open, pending, hold, solved, closed",
			"priority":        "low, normal, high, urgent",
			"type":            "problem, incident, question, task",
			"assignee_id":     "Assignee agent user ID (empty to unassign)",
			"group_id":        "Group ID",
			"organization_id": "Organization ID",
			"tags":            "Comma-separated tags (replaces existing)",
			"custom_fields":   `JSON array of {id, value} custom field objects`,
			"comment":         "Optional comment body to add while updating",
			"public":          "Whether the optional comment is public (default true)",
		},
		Required: []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("zendesk_delete_ticket"), Description: "Delete a Zendesk support ticket. Use after get_ticket. Soft-deletes; the ticket can be restored from deleted tickets.",
		Parameters: map[string]string{"ticket_id": "Ticket ID"},
		Required:   []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("zendesk_list_ticket_comments"), Description: "List comments, replies, and conversation history on a Zendesk ticket. Use after get_ticket when you need the discussion thread.",
		Parameters: map[string]string{
			"ticket_id":   "Ticket ID",
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
			"sort_order":  "asc or desc (default asc)",
		},
		Required: []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("zendesk_add_ticket_comment"), Description: "Add a public or private (internal note) comment/reply to a Zendesk ticket. Use after get_ticket or list_ticket_comments.",
		Parameters: map[string]string{
			"ticket_id": "Ticket ID",
			"body":      "Comment body",
			"public":    "true for public reply, false for internal note (default true)",
		},
		Required: []string{"ticket_id", "body"},
	},
	{
		Name: mcp.ToolName("zendesk_list_ticket_audits"), Description: "List Zendesk ticket audits (status changes, assignments, comment events). Use after get_ticket for the full change history.",
		Parameters: map[string]string{
			"ticket_id":   "Ticket ID",
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
		Required: []string{"ticket_id"},
	},
	{
		Name: mcp.ToolName("zendesk_search"), Description: "Universal Zendesk search across tickets, users, and organizations. Prefer search_tickets when looking only for tickets. Query examples: 'type:user email:a@b.com', 'type:organization name:Acme'.",
		Parameters: map[string]string{
			"query":      "Zendesk search query",
			"sort_by":    "Sort field",
			"sort_order": "asc or desc",
			"page":       "Offset page number (default 1)",
			"per_page":   "Results per page (default 25, max 100)",
		},
		Required: []string{"query"},
	},
	{
		Name: mcp.ToolName("zendesk_list_users"), Description: "List Zendesk users, agents, and end-user customers. Use to find requester_id or assignee_id for ticket workflows.",
		Parameters: map[string]string{
			"role":        "Filter by role: end-user, agent, admin",
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
	},
	{
		Name: mcp.ToolName("zendesk_get_user"), Description: "Get a Zendesk user, agent, or customer by ID. Use after list_users.",
		Parameters: map[string]string{"user_id": "User ID"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("zendesk_get_current_user"), Description: "Get the authenticated Zendesk agent (me). Use to resolve assignee:me and the current user's ID.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("zendesk_create_user"), Description: "Create a Zendesk user, agent, or end-user customer. Use before create_ticket when the requester does not exist.",
		Parameters: map[string]string{
			"name":            "Display name",
			"email":           "Email address",
			"role":            "end-user, agent, admin (default end-user)",
			"phone":           "Phone number",
			"organization_id": "Organization ID",
			"tags":            "Comma-separated tags",
			"verified":        "Mark identity as verified (true/false)",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("zendesk_update_user"), Description: "Update a Zendesk user profile (name, email, role, organization). Use after get_user.",
		Parameters: map[string]string{
			"user_id":         "User ID",
			"name":            "Display name",
			"email":           "Email address",
			"role":            "end-user, agent, admin",
			"phone":           "Phone number",
			"organization_id": "Organization ID",
			"tags":            "Comma-separated tags",
		},
		Required: []string{"user_id"},
	},
	{
		Name: mcp.ToolName("zendesk_list_organizations"), Description: "List Zendesk organizations (customer companies/accounts) used to group end users and tickets.",
		Parameters: map[string]string{
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
	},
	{
		Name: mcp.ToolName("zendesk_get_organization"), Description: "Get a Zendesk organization by ID. Use after list_organizations.",
		Parameters: map[string]string{"organization_id": "Organization ID"},
		Required:   []string{"organization_id"},
	},
	{
		Name: mcp.ToolName("zendesk_list_groups"), Description: "List Zendesk agent groups used for ticket assignment and routing.",
		Parameters: map[string]string{
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
	},
	{
		Name: mcp.ToolName("zendesk_get_group"), Description: "Get a Zendesk agent group by ID. Use after list_groups.",
		Parameters: map[string]string{"group_id": "Group ID"},
		Required:   []string{"group_id"},
	},
	{
		Name: mcp.ToolName("zendesk_list_views"), Description: "List Zendesk ticket views (saved filters such as 'Your unsolved tickets'). Use before list_view_tickets.",
		Parameters: map[string]string{
			"active":      "Filter to active views (true/false)",
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
	},
	{
		Name: mcp.ToolName("zendesk_list_view_tickets"), Description: "List tickets in a Zendesk view. Use after list_views to execute a saved filter.",
		Parameters: map[string]string{
			"view_id":     "View ID",
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
		Required: []string{"view_id"},
	},
	{
		Name: mcp.ToolName("zendesk_list_macros"), Description: "List Zendesk macros (canned ticket update actions). Use to discover macros agents can apply.",
		Parameters: map[string]string{
			"active":      "Filter to active macros (true/false)",
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
	},
	{
		Name: mcp.ToolName("zendesk_search_articles"), Description: "Search Zendesk Help Center knowledge base articles and FAQs. Use for customer-facing documentation, not tickets.",
		Parameters: map[string]string{
			"query":    "Search query",
			"locale":   "Help Center locale (e.g. en-us)",
			"page":     "Offset page number (default 1)",
			"per_page": "Results per page (default 25)",
		},
		Required: []string{"query"},
	},
	{
		Name: mcp.ToolName("zendesk_get_article"), Description: "Get a Zendesk Help Center article by ID including title and body. Use after search_articles.",
		Parameters: map[string]string{
			"article_id": "Article ID",
			"locale":     "Help Center locale (e.g. en-us)",
		},
		Required: []string{"article_id"},
	},
	{
		Name: mcp.ToolName("zendesk_list_tags"), Description: "List tags used on Zendesk tickets, users, and organizations.",
		Parameters: map[string]string{
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
	},
	{
		Name: mcp.ToolName("zendesk_list_satisfaction_ratings"), Description: "List Zendesk CSAT satisfaction ratings for support tickets. Use for customer satisfaction and CSAT reporting.",
		Parameters: map[string]string{
			"score":       "Filter: offered, unoffered, good, bad",
			"start_time":  "Unix timestamp lower bound",
			"end_time":    "Unix timestamp upper bound",
			"page_size":   "Page size (default 25)",
			"page_after":  "Cursor for the next page",
			"page_before": "Cursor for the previous page",
		},
	},
}
