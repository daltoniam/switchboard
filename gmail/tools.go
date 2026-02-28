package gmail

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Profile ─────────────────────────────────────────────────────
	{
		Name: "gmail_get_profile", Description: "Get the current user's Gmail profile (email, messages total, threads total, history ID)",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me' for authenticated user)"},
	},

	// ── Messages ────────────────────────────────────────────────────
	{
		Name: "gmail_list_messages", Description: "List messages in the user's mailbox",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "q": "Gmail search query (same as Gmail search box)", "label_ids": "Comma-separated label IDs to filter by", "max_results": "Max results per page (default 10, max 500)", "page_token": "Token for next page", "include_spam_trash": "Include SPAM and TRASH (true/false)"},
	},
	{
		Name: "gmail_get_message", Description: "Get a specific message by ID",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_id": "Message ID", "format": "Format: full, metadata, minimal, raw (default full)", "metadata_headers": "Comma-separated headers to include when format=metadata"},
		Required:   []string{"message_id"},
	},
	{
		Name: "gmail_send_message", Description: "Send an email message. Provide raw RFC 2822 formatted message or use to/subject/body for simple messages",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "to": "Recipient email address(es), comma-separated", "subject": "Email subject", "body": "Email body (plain text)", "raw": "Base64url-encoded RFC 2822 message (overrides to/subject/body)", "thread_id": "Thread ID to reply to"},
	},
	{
		Name: "gmail_delete_message", Description: "Permanently delete a message (not trash). Cannot be undone",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_id": "Message ID"},
		Required:   []string{"message_id"},
	},
	{
		Name: "gmail_trash_message", Description: "Move a message to the trash",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_id": "Message ID"},
		Required:   []string{"message_id"},
	},
	{
		Name: "gmail_untrash_message", Description: "Remove a message from the trash",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_id": "Message ID"},
		Required:   []string{"message_id"},
	},
	{
		Name: "gmail_modify_message", Description: "Modify labels on a message (add and/or remove labels)",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_id": "Message ID", "add_label_ids": "Comma-separated label IDs to add", "remove_label_ids": "Comma-separated label IDs to remove"},
		Required:   []string{"message_id"},
	},
	{
		Name: "gmail_batch_modify", Description: "Modify labels on multiple messages at once",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_ids": "Comma-separated message IDs", "add_label_ids": "Comma-separated label IDs to add", "remove_label_ids": "Comma-separated label IDs to remove"},
		Required:   []string{"message_ids"},
	},
	{
		Name: "gmail_batch_delete", Description: "Permanently delete multiple messages. Cannot be undone",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_ids": "Comma-separated message IDs"},
		Required:   []string{"message_ids"},
	},
	{
		Name: "gmail_get_attachment", Description: "Get a message attachment by ID",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "message_id": "Message ID", "attachment_id": "Attachment ID"},
		Required:   []string{"message_id", "attachment_id"},
	},

	// ── Threads ─────────────────────────────────────────────────────
	{
		Name: "gmail_list_threads", Description: "List threads in the user's mailbox",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "q": "Gmail search query", "label_ids": "Comma-separated label IDs to filter by", "max_results": "Max results per page (default 10, max 500)", "page_token": "Token for next page", "include_spam_trash": "Include SPAM and TRASH (true/false)"},
	},
	{
		Name: "gmail_get_thread", Description: "Get a specific thread with all its messages",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "thread_id": "Thread ID", "format": "Format: full, metadata, minimal (default full)", "metadata_headers": "Comma-separated headers to include when format=metadata"},
		Required:   []string{"thread_id"},
	},
	{
		Name: "gmail_delete_thread", Description: "Permanently delete a thread (not trash). Cannot be undone",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "thread_id": "Thread ID"},
		Required:   []string{"thread_id"},
	},
	{
		Name: "gmail_trash_thread", Description: "Move a thread to the trash",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "thread_id": "Thread ID"},
		Required:   []string{"thread_id"},
	},
	{
		Name: "gmail_untrash_thread", Description: "Remove a thread from the trash",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "thread_id": "Thread ID"},
		Required:   []string{"thread_id"},
	},
	{
		Name: "gmail_modify_thread", Description: "Modify labels on a thread (add and/or remove labels)",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "thread_id": "Thread ID", "add_label_ids": "Comma-separated label IDs to add", "remove_label_ids": "Comma-separated label IDs to remove"},
		Required:   []string{"thread_id"},
	},

	// ── Labels ──────────────────────────────────────────────────────
	{
		Name: "gmail_list_labels", Description: "List all labels in the user's mailbox",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_get_label", Description: "Get a specific label by ID",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "label_id": "Label ID"},
		Required:   []string{"label_id"},
	},
	{
		Name: "gmail_create_label", Description: "Create a new label",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "name": "Label name", "message_list_visibility": "Visibility in message list: show, hide", "label_list_visibility": "Visibility in label list: labelShow, labelShowIfUnread, labelHide", "background_color": "Background color hex code", "text_color": "Text color hex code"},
		Required:   []string{"name"},
	},
	{
		Name: "gmail_update_label", Description: "Update a label",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "label_id": "Label ID", "name": "New label name", "message_list_visibility": "Visibility in message list: show, hide", "label_list_visibility": "Visibility in label list: labelShow, labelShowIfUnread, labelHide", "background_color": "Background color hex code", "text_color": "Text color hex code"},
		Required:   []string{"label_id"},
	},
	{
		Name: "gmail_delete_label", Description: "Permanently delete a label",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "label_id": "Label ID"},
		Required:   []string{"label_id"},
	},

	// ── Drafts ──────────────────────────────────────────────────────
	{
		Name: "gmail_list_drafts", Description: "List drafts in the user's mailbox",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "q": "Gmail search query", "max_results": "Max results per page", "page_token": "Token for next page", "include_spam_trash": "Include SPAM and TRASH (true/false)"},
	},
	{
		Name: "gmail_get_draft", Description: "Get a specific draft by ID",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "draft_id": "Draft ID", "format": "Format: full, metadata, minimal, raw (default full)"},
		Required:   []string{"draft_id"},
	},
	{
		Name: "gmail_create_draft", Description: "Create a new draft",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "to": "Recipient email address(es), comma-separated", "subject": "Email subject", "body": "Email body (plain text)", "raw": "Base64url-encoded RFC 2822 message (overrides to/subject/body)", "thread_id": "Thread ID for reply drafts"},
	},
	{
		Name: "gmail_update_draft", Description: "Update an existing draft",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "draft_id": "Draft ID", "to": "Recipient email address(es), comma-separated", "subject": "Email subject", "body": "Email body (plain text)", "raw": "Base64url-encoded RFC 2822 message (overrides to/subject/body)", "thread_id": "Thread ID for reply drafts"},
		Required:   []string{"draft_id"},
	},
	{
		Name: "gmail_delete_draft", Description: "Permanently delete a draft",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "draft_id": "Draft ID"},
		Required:   []string{"draft_id"},
	},
	{
		Name: "gmail_send_draft", Description: "Send an existing draft",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "draft_id": "Draft ID"},
		Required:   []string{"draft_id"},
	},

	// ── History ─────────────────────────────────────────────────────
	{
		Name: "gmail_list_history", Description: "List the history of changes to the mailbox since a given history ID",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "start_history_id": "History ID to start listing from (required)", "label_id": "Filter by label ID", "max_results": "Max results per page", "page_token": "Token for next page", "history_types": "Comma-separated types: messageAdded, messageDeleted, labelAdded, labelRemoved"},
		Required:   []string{"start_history_id"},
	},

	// ── Settings ────────────────────────────────────────────────────
	{
		Name: "gmail_get_vacation", Description: "Get vacation responder settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_update_vacation", Description: "Update vacation responder settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "enable_auto_reply": "Enable auto-reply (true/false)", "response_subject": "Auto-reply subject", "response_body_plain_text": "Auto-reply body (plain text)", "response_body_html": "Auto-reply body (HTML)", "restrict_to_contacts": "Only reply to contacts (true/false)", "restrict_to_domain": "Only reply to same domain (true/false)", "start_time": "Start time in milliseconds since epoch", "end_time": "End time in milliseconds since epoch"},
	},
	{
		Name: "gmail_get_auto_forwarding", Description: "Get auto-forwarding settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_update_auto_forwarding", Description: "Update auto-forwarding settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "enabled": "Enable auto-forwarding (true/false)", "email_address": "Email address to forward to", "disposition": "What to do with forwarded messages: leaveInInbox, archive, trash, markRead"},
	},
	{
		Name: "gmail_get_imap", Description: "Get IMAP settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_update_imap", Description: "Update IMAP settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "enabled": "Enable IMAP (true/false)", "auto_expunge": "Auto-expunge (true/false)", "expunge_behavior": "Expunge behavior: archive, deleteForever, trash", "max_folder_size": "Max folder size (0 for no limit)"},
	},
	{
		Name: "gmail_get_pop", Description: "Get POP settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_update_pop", Description: "Update POP settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "access_window": "Access window: disabled, allMail, fromNowOn", "disposition": "What to do after POP fetch: leaveInInbox, archive, trash, markRead"},
	},
	{
		Name: "gmail_get_language", Description: "Get language settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_update_language", Description: "Update language settings",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "display_language": "Display language code (e.g. en, fr, de)"},
		Required:   []string{"display_language"},
	},

	// ── Filters ─────────────────────────────────────────────────────
	{
		Name: "gmail_list_filters", Description: "List all message filters",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_get_filter", Description: "Get a specific message filter",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "filter_id": "Filter ID"},
		Required:   []string{"filter_id"},
	},
	{
		Name: "gmail_create_filter", Description: "Create a new message filter",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "criteria": "JSON string of filter criteria (from, to, subject, query, negatedQuery, hasAttachment, excludeChats, size, sizeComparison)", "action": "JSON string of filter action (addLabelIds, removeLabelIds, forward, sizeComparison)"},
		Required:   []string{"criteria", "action"},
	},
	{
		Name: "gmail_delete_filter", Description: "Delete a message filter",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "filter_id": "Filter ID"},
		Required:   []string{"filter_id"},
	},

	// ── Forwarding Addresses ────────────────────────────────────────
	{
		Name: "gmail_list_forwarding_addresses", Description: "List all forwarding addresses",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_get_forwarding_address", Description: "Get a specific forwarding address",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "forwarding_email": "Forwarding email address"},
		Required:   []string{"forwarding_email"},
	},
	{
		Name: "gmail_create_forwarding_address", Description: "Create a forwarding address (requires verification)",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "forwarding_email": "Email address to add as forwarding address"},
		Required:   []string{"forwarding_email"},
	},
	{
		Name: "gmail_delete_forwarding_address", Description: "Delete a forwarding address",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "forwarding_email": "Forwarding email address to remove"},
		Required:   []string{"forwarding_email"},
	},

	// ── Send As ─────────────────────────────────────────────────────
	{
		Name: "gmail_list_send_as", Description: "List send-as aliases",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_get_send_as", Description: "Get a specific send-as alias",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "send_as_email": "Send-as email address"},
		Required:   []string{"send_as_email"},
	},
	{
		Name: "gmail_create_send_as", Description: "Create a custom 'from' send-as alias",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "send_as_email": "Email address for the alias", "display_name": "Display name for the alias", "reply_to_address": "Reply-to address", "is_default": "Set as default send-as (true/false)"},
		Required:   []string{"send_as_email"},
	},
	{
		Name: "gmail_update_send_as", Description: "Update a send-as alias",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "send_as_email": "Send-as email address", "display_name": "Display name", "reply_to_address": "Reply-to address", "is_default": "Set as default (true/false)"},
		Required:   []string{"send_as_email"},
	},
	{
		Name: "gmail_delete_send_as", Description: "Delete a send-as alias",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "send_as_email": "Send-as email address to delete"},
		Required:   []string{"send_as_email"},
	},
	{
		Name: "gmail_verify_send_as", Description: "Send a verification email to a send-as alias address",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "send_as_email": "Send-as email address to verify"},
		Required:   []string{"send_as_email"},
	},

	// ── Delegates ───────────────────────────────────────────────────
	{
		Name: "gmail_list_delegates", Description: "List delegates for the account",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')"},
	},
	{
		Name: "gmail_get_delegate", Description: "Get a specific delegate",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "delegate_email": "Delegate email address"},
		Required:   []string{"delegate_email"},
	},
	{
		Name: "gmail_create_delegate", Description: "Add a delegate with verification status set to accepted",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "delegate_email": "Email address of the delegate"},
		Required:   []string{"delegate_email"},
	},
	{
		Name: "gmail_delete_delegate", Description: "Remove a delegate",
		Parameters: map[string]string{"user_id": "User ID (defaults to 'me')", "delegate_email": "Delegate email address to remove"},
		Required:   []string{"delegate_email"},
	},
}
