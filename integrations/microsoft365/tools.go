package microsoft365

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("microsoft365_get_me"), Description: "Get the signed-in Microsoft 365 user profile (display name, mail, job title). Start here for Outlook email, calendar meetings, OneDrive files, Teams chats, and To Do tasks.",
		Parameters: map[string]string{
			"select": "OData $select fields (e.g. id,displayName,mail,userPrincipalName)",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_users"), Description: "List Microsoft Entra ID directory users. Needs User.Read.All (admin consent). Prefer search_people for coworker lookup with People.Read.",
		Parameters: map[string]string{
			"filter":    "OData $filter (e.g. startswith(displayName,'Ada'))",
			"select":    "OData $select fields",
			"search":    `OData $search clause(s), e.g. "displayName:Ada" OR "mail:Ada". A bare name is rewritten to those clauses. Requires ConsistencyLevel and User.Read.All.`,
			"top":       "Page size (default 25, max 999)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_search_people"), Description: "Search people and contacts relevant to the signed-in Microsoft 365 user",
		Parameters: map[string]string{
			"search":    "Name or email to search",
			"top":       "Page size (default 25)",
			"select":    "OData $select fields",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_messages"), Description: "List Outlook email messages in a mailbox. Search and find mail using KQL $search or OData $filter.",
		Parameters: map[string]string{
			"user_id":   "User ID or UPN (defaults to me)",
			"folder_id": "Mail folder ID. Omit to list the whole mailbox (Inbox, Sent, Deleted, and others). Use list_mail_folders first.",
			"search":    `KQL search; Graph quotes this automatically (e.g. from:ada@contoso.com subject:budget)`,
			"filter":    "OData $filter (e.g. isRead eq false)",
			"select":    "OData $select fields",
			"orderby":   "OData $orderby (default receivedDateTime desc)",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_get_message"), Description: "Get a specific Outlook email message by ID. Read the full mail body, headers, and attachments. Use after list_messages.",
		Parameters: map[string]string{
			"user_id":    "User ID or UPN (defaults to me)",
			"message_id": "Outlook message ID",
			"select":     "OData $select fields",
		},
		Required: []string{"message_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_send_mail"), Description: "Send an Outlook email message. Compose and deliver mail to recipients.",
		Parameters: map[string]string{
			"user_id":      "User ID or UPN (defaults to me)",
			"to":           "Recipient email address(es), comma-separated",
			"cc":           "CC email address(es), comma-separated",
			"bcc":          "BCC email address(es), comma-separated",
			"subject":      "Email subject",
			"body":         "Email body (plain text by default)",
			"body_type":    "Body content type: Text or HTML (default Text)",
			"save_to_sent": "Save a copy in Sent Items (true/false, default true)",
			"importance":   "low, normal, or high",
		},
		Required: []string{"to", "subject"},
	},
	{
		Name: mcp.ToolName("microsoft365_create_draft"), Description: "Create an Outlook draft email without sending. Use after composing; send later from the mailbox.",
		Parameters: map[string]string{
			"user_id":   "User ID or UPN (defaults to me)",
			"to":        "Recipient email address(es), comma-separated",
			"cc":        "CC email address(es), comma-separated",
			"subject":   "Email subject",
			"body":      "Email body",
			"body_type": "Body content type: Text or HTML (default Text)",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_reply_message"), Description: "Reply to an Outlook email message. Use after get_message.",
		Parameters: map[string]string{
			"user_id":    "User ID or UPN (defaults to me)",
			"message_id": "Outlook message ID to reply to",
			"comment":    "Reply body comment",
			"reply_all":  "If true, reply to all recipients",
		},
		Required: []string{"message_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_delete_message"), Description: "Move an Outlook email message to Deleted Items (soft delete)",
		Parameters: map[string]string{
			"user_id":    "User ID or UPN (defaults to me)",
			"message_id": "Outlook message ID",
		},
		Required: []string{"message_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_list_mail_folders"), Description: "List Outlook mail folders (Inbox, Sent Items, Drafts, custom folders)",
		Parameters: map[string]string{
			"user_id":   "User ID or UPN (defaults to me)",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_calendars"), Description: "List Outlook calendars available to the Microsoft 365 user",
		Parameters: map[string]string{
			"user_id":   "User ID or UPN (defaults to me)",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_events"), Description: "List Outlook calendar events and meetings in a time range. Browse the schedule or find appointments.",
		Parameters: map[string]string{
			"user_id":     "User ID or UPN (defaults to me)",
			"calendar_id": "Calendar ID (defaults to the primary calendar)",
			"start":       "Start of window (ISO 8601). Uses calendarView when set with end.",
			"end":         "End of window (ISO 8601). Uses calendarView when set with start.",
			"filter":      "OData $filter when not using calendarView",
			"select":      "OData $select fields",
			"orderby":     "OData $orderby",
			"top":         "Page size (default 25)",
			"next_link":   "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_get_event"), Description: "Get a single Outlook calendar event by ID. Read meeting details, attendees, location, and Teams join URL. Use after list_events.",
		Parameters: map[string]string{
			"user_id":     "User ID or UPN (defaults to me)",
			"calendar_id": "Calendar ID (defaults to the primary calendar)",
			"event_id":    "Event ID",
			"select":      "OData $select fields",
		},
		Required: []string{"event_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_create_event"), Description: "Create an Outlook calendar event. Schedule a meeting or appointment with attendees, time, and location.",
		Parameters: map[string]string{
			"user_id":     "User ID or UPN (defaults to me)",
			"calendar_id": "Calendar ID (defaults to the primary calendar)",
			"subject":     "Event title",
			"body":        "Event description",
			"body_type":   "Body content type: Text or HTML (default Text)",
			"start":       "Start time ISO 8601 (e.g. 2024-03-15T14:00:00)",
			"end":         "End time ISO 8601",
			"time_zone":   "IANA or Windows time zone (default UTC)",
			"location":    "Event location",
			"attendees":   "Comma-separated attendee emails",
			"is_online":   "If true, create an online Teams meeting",
		},
		Required: []string{"subject", "start", "end"},
	},
	{
		Name: mcp.ToolName("microsoft365_update_event"), Description: "Partially update an Outlook calendar event. Edit title, time, attendees, or location. Use after get_event.",
		Parameters: map[string]string{
			"user_id":     "User ID or UPN (defaults to me)",
			"calendar_id": "Calendar ID (defaults to the primary calendar)",
			"event_id":    "Event ID",
			"subject":     "New title",
			"body":        "New description",
			"body_type":   "Body content type: Text or HTML",
			"start":       "New start time ISO 8601",
			"end":         "New end time ISO 8601",
			"time_zone":   "Time zone for start/end",
			"location":    "New location",
			"attendees":   "Replace attendee list with comma-separated emails",
		},
		Required: []string{"event_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_delete_event"), Description: "Delete an Outlook calendar event or cancel a meeting",
		Parameters: map[string]string{
			"user_id":     "User ID or UPN (defaults to me)",
			"calendar_id": "Calendar ID (defaults to the primary calendar)",
			"event_id":    "Event ID",
		},
		Required: []string{"event_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_list_drive_items"), Description: "List OneDrive and SharePoint files and folders. Start here for finding documents in a drive folder.",
		Parameters: map[string]string{
			"user_id":   "User ID or UPN (defaults to me)",
			"drive_id":  "Drive ID (defaults to the user's OneDrive)",
			"item_id":   "Parent folder item ID (defaults to root)",
			"path":      "Folder path under root (e.g. Documents/Reports)",
			"select":    "OData $select fields",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_get_drive_item"), Description: "Get metadata for a OneDrive or SharePoint file or folder (name, size, webUrl, lastModified). Use download_drive_item for content.",
		Parameters: map[string]string{
			"user_id":  "User ID or UPN (defaults to me)",
			"drive_id": "Drive ID (defaults to the user's OneDrive)",
			"item_id":  "Drive item ID",
			"path":     "File path under root (alternative to item_id)",
			"select":   "OData $select fields",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_search_drive"), Description: "Search OneDrive and SharePoint files by name. Find documents, spreadsheets, and folders.",
		Parameters: map[string]string{
			"user_id":   "User ID or UPN (defaults to me)",
			"drive_id":  "Drive ID (defaults to the user's OneDrive)",
			"q":         "Search query (file name or keyword)",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
		Required: []string{"q"},
	},
	{
		Name: mcp.ToolName("microsoft365_download_drive_item"), Description: "Download OneDrive or SharePoint file content. Text files return inline; binary files return base64.",
		Parameters: map[string]string{
			"user_id":   "User ID or UPN (defaults to me)",
			"drive_id":  "Drive ID (defaults to the user's OneDrive)",
			"item_id":   "Drive item ID",
			"path":      "File path under root (alternative to item_id)",
			"max_bytes": "Cap downloaded bytes (default 5000000, hard max 10000000)",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_create_folder"), Description: "Create a folder in OneDrive or SharePoint",
		Parameters: map[string]string{
			"user_id":  "User ID or UPN (defaults to me)",
			"drive_id": "Drive ID (defaults to the user's OneDrive)",
			"item_id":  "Parent folder item ID (defaults to root)",
			"path":     "Parent folder path under root",
			"name":     "Folder name",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("microsoft365_upload_drive_item"), Description: "Upload or overwrite a small file in OneDrive or SharePoint (simple PUT, under ~4 MB)",
		Parameters: map[string]string{
			"user_id":        "User ID or UPN (defaults to me)",
			"drive_id":       "Drive ID (defaults to the user's OneDrive)",
			"item_id":        "Parent folder item ID (defaults to root)",
			"path":           "Parent folder path under root",
			"name":           "File name",
			"content":        "Text content (mutually exclusive with content_base64)",
			"content_base64": "Base64-encoded binary content",
			"content_type":   "MIME type (default text/plain for content, application/octet-stream for base64)",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("microsoft365_delete_drive_item"), Description: "Delete a OneDrive or SharePoint file or folder",
		Parameters: map[string]string{
			"user_id":  "User ID or UPN (defaults to me)",
			"drive_id": "Drive ID (defaults to the user's OneDrive)",
			"item_id":  "Drive item ID",
			"path":     "File path under root (alternative to item_id)",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_teams"), Description: "List Microsoft Teams the signed-in user is a member of",
		Parameters: map[string]string{
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_channels"), Description: "List channels in a Microsoft Team. Use after list_teams.",
		Parameters: map[string]string{
			"team_id":   "Team ID",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
		Required: []string{"team_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_list_channel_messages"), Description: "List messages in a Microsoft Teams channel. Use after list_channels.",
		Parameters: map[string]string{
			"team_id":    "Team ID",
			"channel_id": "Channel ID",
			"top":        "Page size (default 25)",
			"next_link":  "Full @odata.nextLink URL from a previous page",
		},
		Required: []string{"team_id", "channel_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_send_channel_message"), Description: "Send a message to a Microsoft Teams channel",
		Parameters: map[string]string{
			"team_id":      "Team ID",
			"channel_id":   "Channel ID",
			"content":      "Message body",
			"content_type": "text or html (default text)",
		},
		Required: []string{"team_id", "channel_id", "content"},
	},
	{
		Name: mcp.ToolName("microsoft365_list_chats"), Description: "List Microsoft Teams 1:1 and group chats for the signed-in user",
		Parameters: map[string]string{
			"top":       "Page size (default 25)",
			"filter":    "OData $filter (e.g. chatType eq 'oneOnOne')",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_chat_messages"), Description: "List messages in a Microsoft Teams chat. Use after list_chats.",
		Parameters: map[string]string{
			"chat_id":   "Chat ID",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
		Required: []string{"chat_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_send_chat_message"), Description: "Send a message in a Microsoft Teams chat",
		Parameters: map[string]string{
			"chat_id":      "Chat ID",
			"content":      "Message body",
			"content_type": "text or html (default text)",
		},
		Required: []string{"chat_id", "content"},
	},
	{
		Name: mcp.ToolName("microsoft365_list_todo_lists"), Description: "List Microsoft To Do task lists",
		Parameters: map[string]string{
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
	},
	{
		Name: mcp.ToolName("microsoft365_list_todo_tasks"), Description: "List tasks in a Microsoft To Do list. Use after list_todo_lists.",
		Parameters: map[string]string{
			"list_id":   "To Do list ID",
			"filter":    "OData $filter (e.g. status eq 'notStarted')",
			"top":       "Page size (default 25)",
			"next_link": "Full @odata.nextLink URL from a previous page",
		},
		Required: []string{"list_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_create_todo_task"), Description: "Create a task in a Microsoft To Do list",
		Parameters: map[string]string{
			"list_id":    "To Do list ID",
			"title":      "Task title",
			"body":       "Task notes",
			"due":        "Due date/time ISO 8601",
			"importance": "low, normal, or high",
			"status":     "notStarted, inProgress, completed, waitingOnOthers, deferred",
		},
		Required: []string{"list_id", "title"},
	},
	{
		Name: mcp.ToolName("microsoft365_update_todo_task"), Description: "Update a Microsoft To Do task (title, status, due date). Use after list_todo_tasks.",
		Parameters: map[string]string{
			"list_id":    "To Do list ID",
			"task_id":    "Task ID",
			"title":      "New title",
			"body":       "New notes",
			"due":        "Due date/time ISO 8601",
			"importance": "low, normal, or high",
			"status":     "notStarted, inProgress, completed, waitingOnOthers, deferred",
		},
		Required: []string{"list_id", "task_id"},
	},
	{
		Name: mcp.ToolName("microsoft365_delete_todo_task"), Description: "Delete a Microsoft To Do task",
		Parameters: map[string]string{
			"list_id": "To Do list ID",
			"task_id": "Task ID",
		},
		Required: []string{"list_id", "task_id"},
	},
}
