package paperless

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "paperless_list_documents",
		Description: "List archived Paperless-ngx documents with pagination. Start here to browse a document archive, invoices, receipts, and scanned records.",
		Parameters:  map[string]string{"page": "Page number (default: 1)", "page_size": "Documents per page (default: 25)"},
	},
	{
		Name:        "paperless_search_documents",
		Description: "Search Paperless-ngx archived documents by full-text query, title, or OCR content. Start here to find scanned documents, invoices, receipts, and records.",
		Parameters:  map[string]string{"query": "Full-text search query", "page": "Page number (default: 1)", "page_size": "Documents per page (default: 25)"},
		Required:    []string{"query"},
	},
	{
		Name:        "paperless_create_document",
		Description: "Create and upload a plain-text document to Paperless-ngx for safe live testing. Use title and short non-sensitive content; this creates a real archive document.",
		Parameters:  map[string]string{"title": "Document title", "content": "Plain-text document content"},
		Required:    []string{"title", "content"},
	},
	{
		Name:        "paperless_get_document",
		Description: "Get metadata and OCR text for a specific Paperless-ngx document. Use after listing or searching documents.",
		Parameters:  map[string]string{"document_id": "Document ID"},
		Required:    []string{"document_id"},
	},
	{
		Name:        "paperless_update_document",
		Description: "Update Paperless-ngx document metadata such as title, tags, correspondent, document type, storage path, or custom fields. Use after get_document.",
		Parameters: map[string]string{
			"document_id": "Document ID", "title": "New title", "tags": "Tag ID array", "correspondent": "Correspondent ID", "document_type": "Document type ID", "storage_path": "Storage path ID", "archive_serial_number": "Archive serial number", "created": "Document creation date", "added": "Date added", "custom_fields": "Custom field value array",
		},
		Required: []string{"document_id"},
	},
	{
		Name:        "paperless_delete_document",
		Description: "Permanently delete a Paperless-ngx document. Use after get_document when the document should be removed.",
		Parameters:  map[string]string{"document_id": "Document ID"},
		Required:    []string{"document_id"},
	},
	{
		Name:        "paperless_download_document",
		Description: "Download the original file for a Paperless-ngx document. Use after get_document when the document file is needed.",
		Parameters:  map[string]string{"document_id": "Document ID"},
		Required:    []string{"document_id"},
	},
	{Name: "paperless_list_tags", Description: "List Paperless-ngx tags used to categorize archived documents.", Parameters: map[string]string{}},
	{Name: "paperless_list_correspondents", Description: "List Paperless-ngx correspondents such as vendors, senders, and organizations.", Parameters: map[string]string{}},
	{Name: "paperless_list_document_types", Description: "List Paperless-ngx document types used to classify archived records.", Parameters: map[string]string{}},
	{Name: "paperless_list_storage_paths", Description: "List Paperless-ngx storage paths that organize archived document files.", Parameters: map[string]string{}},
	{Name: "paperless_list_custom_fields", Description: "List Paperless-ngx custom fields available on archived documents.", Parameters: map[string]string{}},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"paperless_list_documents":      listDocuments,
	"paperless_search_documents":    searchDocuments,
	"paperless_create_document":     createDocument,
	"paperless_get_document":        getDocument,
	"paperless_update_document":     updateDocument,
	"paperless_delete_document":     deleteDocument,
	"paperless_download_document":   downloadDocument,
	"paperless_list_tags":           listMetadata("/api/tags/"),
	"paperless_list_correspondents": listMetadata("/api/correspondents/"),
	"paperless_list_document_types": listMetadata("/api/document_types/"),
	"paperless_list_storage_paths":  listMetadata("/api/storage_paths/"),
	"paperless_list_custom_fields":  listMetadata("/api/custom_fields/"),
}
