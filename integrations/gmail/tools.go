package gmail

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Profile ─────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gmail_get_profile"), Description: "Get the current user's Gmail profile (email, messages total, threads total, history ID). Start here to verify access.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me' for authenticated user)"}},
	},

	// ── Messages ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gmail_list_messages"), Description: "List email messages in the user's inbox. Search and find mail using Gmail query syntax.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("q"), Description: "Gmail search query (same as Gmail search box)"}, {Name: mcp.ParamName("label_ids"), Description: "Comma-separated label IDs to filter by"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page (default 10, max 500)"}, {Name: mcp.ParamName("page_token"), Description: "Token for next page"}, {Name: mcp.ParamName("include_spam_trash"), Description: "Include SPAM and TRASH (true/false)"}},
	},
	{
		Name: mcp.ToolName("gmail_get_message"), Description: "Get a specific email message by ID. Read the full mail content, headers, and attachments.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_id"), Description: "Message ID", Required: true}, {Name: mcp.ParamName("format"), Description: "Format: full, metadata, minimal, raw (default full)"}, {Name: mcp.ParamName("metadata_headers"), Description: "Comma-separated headers to include when format=metadata"}},
	},
	{
		Name: mcp.ToolName("gmail_send_message"), Description: "Send an email message. Provide raw RFC 2822 formatted message or use to/subject/body for simple messages",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("from"), Description: "Sender email address (defaults to authenticated user)"}, {Name: mcp.ParamName("to"), Description: "Recipient email address(es), comma-separated"}, {Name: mcp.ParamName("subject"), Description: "Email subject"}, {Name: mcp.ParamName("body"), Description: "Email body (plain text)"}, {Name: mcp.ParamName("raw"), Description: "Base64url-encoded RFC 2822 message (overrides from/to/subject/body)"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID to reply to"}},
	},
	{
		Name: mcp.ToolName("gmail_delete_message"), Description: "Permanently delete a message (not trash). Cannot be undone",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_id"), Description: "Message ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_trash_message"), Description: "Move a message to the trash",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_id"), Description: "Message ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_untrash_message"), Description: "Remove a message from the trash",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_id"), Description: "Message ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_modify_message"), Description: "Modify labels on a message (add and/or remove labels)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_id"), Description: "Message ID", Required: true}, {Name: mcp.ParamName("add_label_ids"), Description: "Comma-separated label IDs to add"}, {Name: mcp.ParamName("remove_label_ids"), Description: "Comma-separated label IDs to remove"}},
	},
	{
		Name: mcp.ToolName("gmail_batch_modify"), Description: "Modify labels on multiple messages at once",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_ids"), Description: "Comma-separated message IDs", Required: true}, {Name: mcp.ParamName("add_label_ids"), Description: "Comma-separated label IDs to add"}, {Name: mcp.ParamName("remove_label_ids"), Description: "Comma-separated label IDs to remove"}},
	},
	{
		Name: mcp.ToolName("gmail_batch_delete"), Description: "Permanently delete multiple messages. Cannot be undone",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_ids"), Description: "Comma-separated message IDs", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_get_attachment"), Description: "Get a message attachment by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("message_id"), Description: "Message ID", Required: true}, {Name: mcp.ParamName("attachment_id"),

		// ── Threads ─────────────────────────────────────────────────────
		Description: "Attachment ID", Required: true}},
	},

	{
		Name: mcp.ToolName("gmail_list_threads"), Description: "List threads in the user's mailbox",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("q"), Description: "Gmail search query"}, {Name: mcp.ParamName("label_ids"), Description: "Comma-separated label IDs to filter by"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page (default 10, max 500)"}, {Name: mcp.ParamName("page_token"), Description: "Token for next page"}, {Name: mcp.ParamName("include_spam_trash"), Description: "Include SPAM and TRASH (true/false)"}},
	},
	{
		Name: mcp.ToolName("gmail_get_thread"), Description: "Get a specific thread with all its messages",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID", Required: true}, {Name: mcp.ParamName("format"), Description: "Format: full, metadata, minimal (default full)"}, {Name: mcp.ParamName("metadata_headers"), Description: "Comma-separated headers to include when format=metadata"}},
	},
	{
		Name: mcp.ToolName("gmail_delete_thread"), Description: "Permanently delete a thread (not trash). Cannot be undone",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_trash_thread"), Description: "Move a thread to the trash",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_untrash_thread"), Description: "Remove a thread from the trash",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_modify_thread"), Description: "Modify labels on a thread (add and/or remove labels)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID", Required: true}, {Name: mcp.ParamName("add_label_ids"), Description: "Comma-separated label IDs to add"},

		// ── Labels ──────────────────────────────────────────────────────
		{Name: mcp.ParamName("remove_label_ids"), Description: "Comma-separated label IDs to remove"}},
	},

	{
		Name: mcp.ToolName("gmail_list_labels"), Description: "List all labels in the user's mailbox",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_get_label"), Description: "Get a specific label by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("label_id"), Description: "Label ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_create_label"), Description: "Create a new label",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("name"), Description: "Label name", Required: true}, {Name: mcp.ParamName("message_list_visibility"), Description: "Visibility in message list: show, hide"}, {Name: mcp.ParamName("label_list_visibility"), Description: "Visibility in label list: labelShow, labelShowIfUnread, labelHide"}, {Name: mcp.ParamName("background_color"), Description: "Background color hex code"}, {Name: mcp.ParamName("text_color"), Description: "Text color hex code"}},
	},
	{
		Name: mcp.ToolName("gmail_update_label"), Description: "Update a label",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("label_id"), Description: "Label ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New label name"}, {Name: mcp.ParamName("message_list_visibility"), Description: "Visibility in message list: show, hide"}, {Name: mcp.ParamName("label_list_visibility"), Description: "Visibility in label list: labelShow, labelShowIfUnread, labelHide"}, {Name: mcp.ParamName("background_color"), Description: "Background color hex code"}, {Name: mcp.ParamName("text_color"), Description: "Text color hex code"}},
	},
	{
		Name: mcp.ToolName("gmail_delete_label"), Description: "Permanently delete a label",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("label_id"), Description:

		// ── Drafts ──────────────────────────────────────────────────────
		"Label ID", Required: true}},
	},

	{
		Name: mcp.ToolName("gmail_list_drafts"), Description: "List email drafts in the user's mailbox. View unsent composed messages.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("q"), Description: "Gmail search query"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}, {Name: mcp.ParamName("page_token"), Description: "Token for next page"}, {Name: mcp.ParamName("include_spam_trash"), Description: "Include SPAM and TRASH (true/false)"}},
	},
	{
		Name: mcp.ToolName("gmail_get_draft"), Description: "Get a specific draft by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("draft_id"), Description: "Draft ID", Required: true}, {Name: mcp.ParamName("format"), Description: "Format: full, metadata, minimal, raw (default full)"}},
	},
	{
		Name: mcp.ToolName("gmail_create_draft"), Description: "Create a new email draft. Compose and write a message to send later or save as a reply draft.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("to"), Description: "Recipient email address(es), comma-separated"}, {Name: mcp.ParamName("subject"), Description: "Email subject"}, {Name: mcp.ParamName("body"), Description: "Email body (plain text)"}, {Name: mcp.ParamName("raw"), Description: "Base64url-encoded RFC 2822 message (overrides to/subject/body)"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID for reply drafts"}},
	},
	{
		Name: mcp.ToolName("gmail_update_draft"), Description: "Update an existing email draft. Edit the composed mail message before sending.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("draft_id"), Description: "Draft ID", Required: true}, {Name: mcp.ParamName("to"), Description: "Recipient email address(es), comma-separated"}, {Name: mcp.ParamName("subject"), Description: "Email subject"}, {Name: mcp.ParamName("body"), Description: "Email body (plain text)"}, {Name: mcp.ParamName("raw"), Description: "Base64url-encoded RFC 2822 message (overrides to/subject/body)"}, {Name: mcp.ParamName("thread_id"), Description: "Thread ID for reply drafts"}},
	},
	{
		Name: mcp.ToolName("gmail_delete_draft"), Description: "Permanently delete a draft",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("draft_id"), Description: "Draft ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_send_draft"), Description: "Send an existing email draft. Deliver a previously composed mail message.",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("draft_id"), Description:

		// ── History ─────────────────────────────────────────────────────
		"Draft ID", Required: true}},
	},

	{
		Name: mcp.ToolName("gmail_list_history"), Description: "List the history of changes to the mailbox since a given history ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("start_history_id"), Description: "History ID to start listing from (required)", Required: true}, {Name: mcp.ParamName("label_id"), Description: "Filter by label ID"}, {Name: mcp.ParamName("max_results"), Description: "Max results per page"}, {Name: mcp.ParamName("page_token"), Description: "Token for next page"},

		// ── Settings ────────────────────────────────────────────────────
		{Name: mcp.ParamName("history_types"), Description: "Comma-separated types: messageAdded, messageDeleted, labelAdded, labelRemoved"}},
	},

	{
		Name: mcp.ToolName("gmail_get_vacation"), Description: "Get vacation responder settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_update_vacation"), Description: "Update vacation responder settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("enable_auto_reply"), Description: "Enable auto-reply (true/false)"}, {Name: mcp.ParamName("response_subject"), Description: "Auto-reply subject"}, {Name: mcp.ParamName("response_body_plain_text"), Description: "Auto-reply body (plain text)"}, {Name: mcp.ParamName("response_body_html"), Description: "Auto-reply body (HTML)"}, {Name: mcp.ParamName("restrict_to_contacts"), Description: "Only reply to contacts (true/false)"}, {Name: mcp.ParamName("restrict_to_domain"), Description: "Only reply to same domain (true/false)"}, {Name: mcp.ParamName("start_time"), Description: "Start time in milliseconds since epoch"}, {Name: mcp.ParamName("end_time"), Description: "End time in milliseconds since epoch"}},
	},
	{
		Name: mcp.ToolName("gmail_get_auto_forwarding"), Description: "Get auto-forwarding settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_update_auto_forwarding"), Description: "Update auto-forwarding settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("enabled"), Description: "Enable auto-forwarding (true/false)"}, {Name: mcp.ParamName("email_address"), Description: "Email address to forward to"}, {Name: mcp.ParamName("disposition"), Description: "What to do with forwarded messages: leaveInInbox, archive, trash, markRead"}},
	},
	{
		Name: mcp.ToolName("gmail_get_imap"), Description: "Get IMAP settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_update_imap"), Description: "Update IMAP settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("enabled"), Description: "Enable IMAP (true/false)"}, {Name: mcp.ParamName("auto_expunge"), Description: "Auto-expunge (true/false)"}, {Name: mcp.ParamName("expunge_behavior"), Description: "Expunge behavior: archive, deleteForever, trash"}, {Name: mcp.ParamName("max_folder_size"), Description: "Max folder size (0 for no limit)"}},
	},
	{
		Name: mcp.ToolName("gmail_get_pop"), Description: "Get POP settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_update_pop"), Description: "Update POP settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("access_window"), Description: "Access window: disabled, allMail, fromNowOn"}, {Name: mcp.ParamName("disposition"), Description: "What to do after POP fetch: leaveInInbox, archive, trash, markRead"}},
	},
	{
		Name: mcp.ToolName("gmail_get_language"), Description: "Get language settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_update_language"), Description: "Update language settings",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("display_language"), Description: "Display language code (e.g. en, fr, de)",

		// ── Filters ─────────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("gmail_list_filters"), Description: "List all message filters",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_get_filter"), Description: "Get a specific message filter",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("filter_id"), Description: "Filter ID", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_create_filter"), Description: "Create a new message filter",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("criteria"), Description: "JSON string of filter criteria (from, to, subject, query, negatedQuery, hasAttachment, excludeChats, size, sizeComparison)", Required: true}, {Name: mcp.ParamName("action"), Description: "JSON string of filter action (addLabelIds, removeLabelIds, forward, sizeComparison)", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_delete_filter"), Description: "Delete a message filter",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("filter_id"), Description:

		// ── Forwarding Addresses ────────────────────────────────────────
		"Filter ID", Required: true}},
	},

	{
		Name: mcp.ToolName("gmail_list_forwarding_addresses"), Description: "List all forwarding addresses",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_get_forwarding_address"), Description: "Get a specific forwarding address",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("forwarding_email"), Description: "Forwarding email address", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_create_forwarding_address"), Description: "Create a forwarding address (requires verification)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("forwarding_email"), Description: "Email address to add as forwarding address", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_delete_forwarding_address"), Description: "Delete a forwarding address",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("forwarding_email"), Description: "Forwarding email address to remove",

		// ── Send As ─────────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("gmail_list_send_as"), Description: "List send-as aliases",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_get_send_as"), Description: "Get a specific send-as alias",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("send_as_email"), Description: "Send-as email address", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_create_send_as"), Description: "Create a custom 'from' send-as alias",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("send_as_email"), Description: "Email address for the alias", Required: true}, {Name: mcp.ParamName("display_name"), Description: "Display name for the alias"}, {Name: mcp.ParamName("reply_to_address"), Description: "Reply-to address"}, {Name: mcp.ParamName("is_default"), Description: "Set as default send-as (true/false)"}},
	},
	{
		Name: mcp.ToolName("gmail_update_send_as"), Description: "Update a send-as alias",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("send_as_email"), Description: "Send-as email address", Required: true}, {Name: mcp.ParamName("display_name"), Description: "Display name"}, {Name: mcp.ParamName("reply_to_address"), Description: "Reply-to address"}, {Name: mcp.ParamName("is_default"), Description: "Set as default (true/false)"}},
	},
	{
		Name: mcp.ToolName("gmail_delete_send_as"), Description: "Delete a send-as alias",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("send_as_email"), Description: "Send-as email address to delete", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_verify_send_as"), Description: "Send a verification email to a send-as alias address",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("send_as_email"), Description: "Send-as email address to verify",

		// ── Delegates ───────────────────────────────────────────────────
		Required: true}},
	},

	{
		Name: mcp.ToolName("gmail_list_delegates"), Description: "List delegates for the account",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}},
	},
	{
		Name: mcp.ToolName("gmail_get_delegate"), Description: "Get a specific delegate",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("delegate_email"), Description: "Delegate email address", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_create_delegate"), Description: "Add a delegate with verification status set to accepted",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("delegate_email"), Description: "Email address of the delegate", Required: true}},
	},
	{
		Name: mcp.ToolName("gmail_delete_delegate"), Description: "Remove a delegate",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("user_id"), Description: "User ID (defaults to 'me')"}, {Name: mcp.ParamName("delegate_email"), Description: "Delegate email address to remove", Required: true}},
	},
}
