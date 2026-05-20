package gdrive

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Files: search & read ────────────────────────────────────────
	{
		Name: mcp.ToolName("gdrive_list_files"), Description: "Search and list files and folders in Drive. Start here for finding documents, photos, folders, or anything else by name, owner, or type. Uses Drive's expressive query syntax (e.g. \"name contains 'budget'\", \"mimeType='application/vnd.google-apps.folder'\").",
		Parameters: map[string]string{
			"q":                             "Drive search query (e.g. \"name contains 'report'\", \"'<folder_id>' in parents\", \"mimeType='application/vnd.google-apps.document'\", \"trashed=false\"). See https://developers.google.com/drive/api/guides/search-files",
			"page_size":                     "Max results per page (default 100, max 1000)",
			"page_token":                    "Token for next page",
			"order_by":                      "Comma-separated sort keys with optional 'desc' suffix: createdTime, folder, modifiedByMeTime, modifiedTime, name, name_natural, quotaBytesUsed, recency, sharedWithMeTime, starred, viewedByMeTime",
			"fields":                        "Selector for response fields (e.g. 'nextPageToken,files(id,name,mimeType,parents)'). Defaults to a useful subset",
			"corpora":                       "Bodies of items to query: user, drive, allDrives, domain",
			"drive_id":                      "ID of the shared drive to search (required when corpora=drive)",
			"include_items_from_all_drives": "Include items from shared drives (true/false, requires supports_all_drives=true)",
			"spaces":                        "Comma-separated spaces: drive, appDataFolder",
			"supports_all_drives":           "true/false (default true)",
		},
	},
	{
		Name: mcp.ToolName("gdrive_get_file"), Description: "Get metadata for a single file or folder (name, mimeType, size, parents, modifiedTime, owners, permissions, etc.). Use this for everything except the file content itself — for content use gdrive_download_file or gdrive_export_file.",
		Parameters: map[string]string{
			"file_id":             "File or folder ID",
			"fields":              "Field selector (default '*' returns all). Example: 'id,name,mimeType,parents,size,modifiedTime'",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_download_file"), Description: "Download the binary content of a non-Google file (PDFs, images, ZIPs, CSV, etc.). For Google Docs/Sheets/Slides use gdrive_export_file. Response is a JSON envelope with base64-encoded content plus content_type, or text content inline when content_type is text/*.",
		Parameters: map[string]string{
			"file_id":             "File ID",
			"acknowledge_abuse":   "Set to true to download a file flagged as malicious",
			"supports_all_drives": "true/false (default true)",
			"max_bytes":           "Cap downloaded bytes (default 5_000_000, hard max 10_000_000)",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_export_file"), Description: "Export a Google Docs Editors file (Docs, Sheets, Slides, Drawings) as a different MIME type. Common export types: 'application/pdf', 'text/plain', 'text/csv', 'text/markdown' (for Docs), 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' (.docx). Response is a JSON envelope with content_type plus either base64 content or inline text when content_type is text/*.",
		Parameters: map[string]string{
			"file_id":   "Google Docs/Sheets/Slides/Drawings file ID",
			"mime_type": "Target MIME type to export as",
			"max_bytes": "Cap exported bytes (default 5_000_000, hard max 10_000_000)",
		},
		Required: []string{"file_id", "mime_type"},
	},

	// ── Files: write ────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gdrive_create_file"), Description: "Create a new file or upload content. Two modes: (1) metadata-only — pass name/mime_type/parents; (2) with content — pass content (UTF-8 string) or content_base64 (binary). Use gdrive_create_folder for folders specifically.",
		Parameters: map[string]string{
			"name":                "File name",
			"mime_type":           "MIME type (e.g. 'text/plain', 'application/pdf', 'application/vnd.google-apps.document' to create a new Google Doc from plain text content)",
			"parents":             "Comma-separated parent folder IDs (omit for My Drive root)",
			"description":         "File description",
			"content":             "Text content (mutually exclusive with content_base64)",
			"content_base64":      "Base64-encoded binary content (mutually exclusive with content)",
			"starred":             "true/false",
			"app_properties":      "JSON object string for appProperties",
			"properties":          "JSON object string for properties",
			"body":                "Raw JSON file metadata body — overrides convenience args except content/content_base64",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("gdrive_update_file"), Description: "Update an existing file's metadata and/or content. To rename, change parents, or update content. Use gdrive_trash_file for soft delete.",
		Parameters: map[string]string{
			"file_id":             "File ID",
			"name":                "New name",
			"mime_type":           "New MIME type",
			"description":         "New description",
			"starred":             "true/false",
			"trashed":             "true/false (prefer gdrive_trash_file)",
			"add_parents":         "Comma-separated parent IDs to add",
			"remove_parents":      "Comma-separated parent IDs to remove",
			"content":             "Text content to overwrite the file with",
			"content_base64":      "Base64-encoded binary content to overwrite the file with",
			"body":                "Raw JSON file metadata body — overrides convenience args",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_copy_file"), Description: "Copy a file to a new file (metadata + content). Note: cannot copy folders (use gdrive_list_files + repeated copies).",
		Parameters: map[string]string{
			"file_id":             "Source file ID",
			"name":                "New file name (defaults to 'Copy of <original>')",
			"parents":             "Comma-separated parent folder IDs",
			"description":         "New description",
			"body":                "Raw JSON metadata body — overrides convenience args",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_delete_file"), Description: "Permanently delete a file (no trash). Cannot be undone. For soft delete use gdrive_trash_file.",
		Parameters: map[string]string{
			"file_id":             "File ID",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_trash_file"), Description: "Move a file to the trash (soft delete, reversible via gdrive_untrash_file).",
		Parameters: map[string]string{
			"file_id":             "File ID",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_untrash_file"), Description: "Restore a file from the trash.",
		Parameters: map[string]string{
			"file_id":             "File ID",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_empty_trash"), Description: "Permanently delete all files in the user's trash. Cannot be undone.",
		Parameters: map[string]string{
			"drive_id": "Shared drive ID (omit to empty My Drive trash)",
		},
	},
	{
		Name: mcp.ToolName("gdrive_create_folder"), Description: "Create a new folder. Convenience wrapper around gdrive_create_file with mime_type set to application/vnd.google-apps.folder.",
		Parameters: map[string]string{
			"name":                "Folder name",
			"parents":             "Comma-separated parent folder IDs (omit for My Drive root)",
			"description":         "Folder description",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("gdrive_generate_ids"), Description: "Generate a set of file IDs that can be used in subsequent insert/copy requests. Useful for pre-allocating IDs before uploading.",
		Parameters: map[string]string{
			"count": "Number of IDs to generate (default 10, max 1000)",
			"space": "Space the IDs are for: drive (default) or appDataFolder",
			"type":  "Resource type the IDs are for: files (default) or shortcuts",
		},
	},

	// ── Permissions (sharing) ───────────────────────────────────────
	{
		Name: mcp.ToolName("gdrive_list_permissions"), Description: "List the sharing permissions on a file or shared drive. Shows who has access at what role.",
		Parameters: map[string]string{
			"file_id":                 "File or shared drive ID",
			"page_size":               "Max results per page",
			"page_token":              "Token for next page",
			"supports_all_drives":     "true/false (default true)",
			"use_domain_admin_access": "true/false — issue as domain administrator",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_get_permission"), Description: "Get a specific permission entry by ID.",
		Parameters: map[string]string{
			"file_id":             "File or shared drive ID",
			"permission_id":       "Permission ID",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id", "permission_id"},
	},
	{
		Name: mcp.ToolName("gdrive_create_permission"), Description: "Share a file or folder. Grants a role to a user, group, domain, or anyone. By default sends an email notification.",
		Parameters: map[string]string{
			"file_id":                 "File or shared drive ID",
			"role":                    "Role: owner, organizer, fileOrganizer, writer, commenter, reader",
			"type":                    "Grantee type: user, group, domain, anyone",
			"email_address":           "Email (required for user/group)",
			"domain":                  "Domain (required for type=domain)",
			"allow_file_discovery":    "true/false (for type=domain or anyone)",
			"send_notification_email": "true/false (default true for user/group)",
			"email_message":           "Custom message to include in the notification email",
			"transfer_ownership":      "true/false — required when changing owner",
			"move_to_new_owners_root": "true/false — when transferring ownership in My Drive",
			"supports_all_drives":     "true/false (default true)",
			"body":                    "Raw JSON permission body — overrides convenience args",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_update_permission"), Description: "Update a permission's role.",
		Parameters: map[string]string{
			"file_id":             "File or shared drive ID",
			"permission_id":       "Permission ID",
			"role":                "New role: owner, organizer, fileOrganizer, writer, commenter, reader",
			"expiration_time":     "RFC3339 expiration timestamp",
			"transfer_ownership":  "true/false",
			"supports_all_drives": "true/false (default true)",
			"body":                "Raw JSON patch body — overrides convenience args",
		},
		Required: []string{"file_id", "permission_id"},
	},
	{
		Name: mcp.ToolName("gdrive_delete_permission"), Description: "Revoke a permission (unshare).",
		Parameters: map[string]string{
			"file_id":             "File or shared drive ID",
			"permission_id":       "Permission ID",
			"supports_all_drives": "true/false (default true)",
		},
		Required: []string{"file_id", "permission_id"},
	},

	// ── Revisions (file version history) ────────────────────────────
	{
		Name: mcp.ToolName("gdrive_list_revisions"), Description: "List a file's revision history.",
		Parameters: map[string]string{
			"file_id":    "File ID",
			"page_size":  "Max results per page",
			"page_token": "Token for next page",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_get_revision"), Description: "Get a specific revision's metadata.",
		Parameters: map[string]string{
			"file_id":     "File ID",
			"revision_id": "Revision ID",
			"fields":      "Field selector",
		},
		Required: []string{"file_id", "revision_id"},
	},
	{
		Name: mcp.ToolName("gdrive_update_revision"), Description: "Update a revision (e.g. keepForever to pin it from auto-cleanup).",
		Parameters: map[string]string{
			"file_id":                  "File ID",
			"revision_id":              "Revision ID",
			"keep_forever":             "true/false — pin from auto-cleanup",
			"published":                "true/false — publish revision",
			"publish_auto":             "true/false — auto-publish subsequent revisions",
			"published_outside_domain": "true/false",
			"body":                     "Raw JSON patch body — overrides convenience args",
		},
		Required: []string{"file_id", "revision_id"},
	},
	{
		Name: mcp.ToolName("gdrive_delete_revision"), Description: "Permanently delete a file revision.",
		Parameters: map[string]string{
			"file_id":     "File ID",
			"revision_id": "Revision ID",
		},
		Required: []string{"file_id", "revision_id"},
	},

	// ── Comments + replies ──────────────────────────────────────────
	{
		Name: mcp.ToolName("gdrive_list_comments"), Description: "List comments on a file.",
		Parameters: map[string]string{
			"file_id":             "File ID",
			"page_size":           "Max results per page",
			"page_token":          "Token for next page",
			"include_deleted":     "true/false",
			"start_modified_time": "RFC3339 — only comments modified at/after this time",
		},
		Required: []string{"file_id"},
	},
	{
		Name: mcp.ToolName("gdrive_get_comment"), Description: "Get a comment by ID, including its replies.",
		Parameters: map[string]string{
			"file_id":         "File ID",
			"comment_id":      "Comment ID",
			"include_deleted": "true/false",
		},
		Required: []string{"file_id", "comment_id"},
	},
	{
		Name: mcp.ToolName("gdrive_create_comment"), Description: "Create a new comment on a file. Optionally anchored to a specific quote/range.",
		Parameters: map[string]string{
			"file_id":             "File ID",
			"content":             "Comment body (plain text or HTML)",
			"anchor":              "Anchor JSON string — for anchored comments",
			"quoted_file_content": "JSON object string with mimeType+value the comment quotes",
			"body":                "Raw JSON comment body — overrides convenience args",
		},
		Required: []string{"file_id", "content"},
	},
	{
		Name: mcp.ToolName("gdrive_update_comment"), Description: "Update a comment's content or resolved status.",
		Parameters: map[string]string{
			"file_id":    "File ID",
			"comment_id": "Comment ID",
			"content":    "New content",
			"resolved":   "true/false",
			"body":       "Raw JSON patch body — overrides convenience args",
		},
		Required: []string{"file_id", "comment_id"},
	},
	{
		Name: mcp.ToolName("gdrive_delete_comment"), Description: "Delete a comment.",
		Parameters: map[string]string{
			"file_id":    "File ID",
			"comment_id": "Comment ID",
		},
		Required: []string{"file_id", "comment_id"},
	},
	{
		Name: mcp.ToolName("gdrive_create_reply"), Description: "Reply to a comment.",
		Parameters: map[string]string{
			"file_id":    "File ID",
			"comment_id": "Comment ID",
			"content":    "Reply content",
			"action":     "Action: resolve or reopen (optional)",
			"body":       "Raw JSON reply body — overrides convenience args",
		},
		Required: []string{"file_id", "comment_id", "content"},
	},

	// ── Shared drives ───────────────────────────────────────────────
	{
		Name: mcp.ToolName("gdrive_list_drives"), Description: "List the shared drives the user has access to.",
		Parameters: map[string]string{
			"page_size":               "Max results per page",
			"page_token":              "Token for next page",
			"q":                       "Search query (e.g. \"name contains 'Engineering'\")",
			"use_domain_admin_access": "true/false — list all shared drives in the domain",
		},
	},
	{
		Name: mcp.ToolName("gdrive_get_drive"), Description: "Get metadata for a single shared drive.",
		Parameters: map[string]string{
			"drive_id":                "Shared drive ID",
			"use_domain_admin_access": "true/false",
		},
		Required: []string{"drive_id"},
	},
	{
		Name: mcp.ToolName("gdrive_create_drive"), Description: "Create a new shared drive.",
		Parameters: map[string]string{
			"name":       "Shared drive name",
			"request_id": "Unique idempotency key (auto-generated if omitted)",
			"theme_id":   "Theme ID — for the drive cover image",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("gdrive_update_drive"), Description: "Update a shared drive's metadata (name, theme, restrictions).",
		Parameters: map[string]string{
			"drive_id":                "Shared drive ID",
			"name":                    "New name",
			"theme_id":                "New theme ID",
			"body":                    "Raw JSON patch body — overrides convenience args (use for restrictions, hidden, etc.)",
			"use_domain_admin_access": "true/false",
		},
		Required: []string{"drive_id"},
	},
	{
		Name: mcp.ToolName("gdrive_delete_drive"), Description: "Delete a shared drive. The drive must be empty.",
		Parameters: map[string]string{
			"drive_id":                "Shared drive ID",
			"allow_item_deletion":     "true/false — delete non-empty drive (domain admin only)",
			"use_domain_admin_access": "true/false",
		},
		Required: []string{"drive_id"},
	},
	{
		Name: mcp.ToolName("gdrive_hide_drive"), Description: "Hide a shared drive from the user's default view.",
		Parameters: map[string]string{
			"drive_id": "Shared drive ID",
		},
		Required: []string{"drive_id"},
	},
	{
		Name: mcp.ToolName("gdrive_unhide_drive"), Description: "Restore a shared drive to the user's default view.",
		Parameters: map[string]string{
			"drive_id": "Shared drive ID",
		},
		Required: []string{"drive_id"},
	},

	// ── About ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("gdrive_get_about"), Description: "Get information about the authenticated user and their Drive (storage quota, supported MIME types, etc.).",
		Parameters: map[string]string{
			"fields": "Field selector (default '*'). Example: 'user,storageQuota'",
		},
	},
}
