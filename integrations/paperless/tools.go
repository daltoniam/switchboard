package paperless

import (
	"maps"

	mcp "github.com/daltoniam/switchboard"
)

var tools = buildTools()

var (
	pageParameters = map[string]string{"page": "Page number (default: 1)", "page_size": "Results per page (default: 25)"}
	idParameter    = map[string]string{"id": "Resource ID"}
	dataParameter  = map[string]string{"data": "JSON object sent to the documented Paperless API resource"}
)

type configurationResource struct {
	singular string
	plural   string
	purpose  string
	danger   string
}

var configurationResources = []configurationResource{
	{"tag", "tags", "categorize archived documents", "Changing ownership or set_permissions can grant or revoke access to tagged documents."},
	{"correspondent", "correspondents", "identify vendors, senders, and organizations", "Changing ownership or set_permissions can grant or revoke access to matching document metadata."},
	{"document_type", "document_types", "classify archived records", "Changing ownership or set_permissions can grant or revoke access to document metadata."},
	{"storage_path", "storage_paths", "organize archived document files", "Updating a storage path can asynchronously rename or move documents; ownership or set_permissions changes affect access."},
	{"custom_field", "custom_fields", "store structured metadata on documents", "Changing or deleting a field can affect document metadata and its available values."},
	{"saved_view", "saved_views", "reuse document search filters", "Changing ownership or set_permissions can share or restrict access to the saved view."},
	{"mail_rule", "mail_rules", "route and process imported email", "Changes affect how future imported email is matched and processed."},
	{"user", "users", "administer Paperless user accounts", "Security-sensitive: data may set passwords, staff or superuser privileges, group memberships, and account activation; changes immediately alter account access, and deletion removes the account and its access."},
	{"group", "groups", "administer Paperless permission groups", "Security-sensitive: data may grant Django permissions and change member access to Paperless resources; deletion revokes access granted through the group."},
}

func buildTools() []mcp.ToolDefinition {
	definitions := []mcp.ToolDefinition{
		{Name: "paperless_list_documents", Description: "List archived Paperless-ngx documents with pagination. Start here to browse a document archive, invoices, receipts, and scanned records.", Parameters: pageParameters},
		{Name: "paperless_search_documents", Description: "Search Paperless-ngx archived documents by full-text query, title, or OCR content. Start here to find scanned documents, invoices, receipts, and records.", Parameters: map[string]string{"query": "Full-text search query", "page": "Page number (default: 1)", "page_size": "Documents per page (default: 25)"}, Required: []string{"query"}},
		{Name: "paperless_create_document", Description: "Create and upload a plain-text document to Paperless-ngx for safe live testing. Use title and short non-sensitive content; this creates a real archive document.", Parameters: map[string]string{"title": "Document title", "content": "Plain-text document content"}, Required: []string{"title", "content"}},
		{Name: "paperless_get_document", Description: "Get metadata for a specific Paperless-ngx document, without OCR text. Use after listing or searching documents.", Parameters: map[string]string{"document_id": "Document ID"}, Required: []string{"document_id"}},
		{Name: "paperless_get_document_ocr_text", Description: "Get a paginated slice of OCR text for a Paperless-ngx document. Use after get_document when document text is needed; continue with next_offset while has_more is true.", Parameters: map[string]string{"document_id": "Document ID", "offset": "Zero-based OCR character offset (default: 0)", "limit": "OCR characters to return (default: 10000, maximum: 20000)"}, Required: []string{"document_id"}},

		{Name: "paperless_update_document", Description: "Update Paperless-ngx document metadata such as title, tags, correspondent, document type, storage path, or custom fields. Use after get_document.", Parameters: map[string]string{"document_id": "Document ID", "title": "New title", "tags": "Tag ID array", "correspondent": "Correspondent ID", "document_type": "Document type ID", "storage_path": "Storage path ID", "archive_serial_number": "Archive serial number", "created": "Document creation date", "added": "Date added", "custom_fields": "Custom field value array"}, Required: []string{"document_id"}},
		{Name: "paperless_delete_document", Description: "Permanently delete a Paperless-ngx document. Use after get_document when the document should be removed.", Parameters: map[string]string{"document_id": "Document ID"}, Required: []string{"document_id"}},
		{Name: "paperless_download_document", Description: "Download the original file for a Paperless-ngx document as a JSON envelope with content type, byte count, and base64 content. Use after get_document when the document file is needed.", Parameters: map[string]string{"document_id": "Document ID"}, Required: []string{"document_id"}},

		{Name: "paperless_list_tasks", Description: "List asynchronous Paperless-ngx background tasks, including document consumption and workflow jobs. Start here to monitor task execution.", Parameters: pageParameters},
		{Name: "paperless_get_task", Description: "Get a specific Paperless-ngx asynchronous task by ID. Use after list_tasks or an upload returns a task ID.", Parameters: idParameter, Required: []string{"id"}},
		{Name: "paperless_get_task_summary", Description: "Get aggregate Paperless-ngx background-task execution statistics over recent days. Use for administration and task health monitoring.", Parameters: map[string]string{"days": "Days of task history (default: 30)"}},
		{Name: "paperless_get_task_status_counts", Description: "Get counts of Paperless-ngx tasks in the upstream all, needs_attention, in_progress, and completed sections. Use for task health monitoring."},
		{Name: "paperless_list_active_tasks", Description: "List currently pending and running Paperless-ngx background tasks. Use to investigate active document consumption or workflows."},
		{Name: "paperless_run_task", Description: "Manually dispatch a supported Paperless-ngx background task. Requires a superuser and causes real server work; use only when explicitly requested.", Parameters: map[string]string{"task_type": "Documented Paperless task type"}, Required: []string{"task_type"}},
		{Name: "paperless_acknowledge_tasks", Description: "Acknowledge Paperless-ngx task records to clear task attention state. This changes task management state; use only after reviewing tasks.", Parameters: dataParameter, Required: []string{"data"}},
		{Name: "paperless_bulk_edit_documents", Description: "Run a documented asynchronous bulk operation on Paperless-ngx documents. Supports metadata, tags, permissions, reprocessing, and deletion; deletion is destructive, so use only when explicitly requested after listing documents.", Parameters: dataParameter, Required: []string{"data"}},
		{Name: "paperless_bulk_edit_objects", Description: "Run a documented bulk permission or deletion operation for Paperless-ngx tags, correspondents, document types, or storage paths. Deletion is destructive; use only when explicitly requested after listing objects.", Parameters: dataParameter, Required: []string{"data"}},
	}
	definitions = append(definitions, workflowTools()...)
	for _, resource := range configurationResources {
		definitions = append(definitions, resourceTools(resource)...)
	}
	return definitions
}

func workflowTools() []mcp.ToolDefinition {
	return []mcp.ToolDefinition{
		{Name: "paperless_list_workflows", Description: "List Paperless-ngx workflows. Workflow triggers and actions are nested components, not standalone resources.", Parameters: pageParameters},
		{Name: "paperless_get_workflow", Description: "Get a Paperless-ngx workflow with its nested trigger and action payloads. Use before changing automation.", Parameters: idParameter, Required: []string{"id"}},
		{Name: "paperless_create_workflow", Description: "Create Paperless-ngx document automation. data must contain name, nested triggers, and nested actions using Paperless numeric type values (trigger 1=consumption, 2=document added, 3=document updated, 4=scheduled; action 1=assignment, 2=removal, 3=email, 4=webhook, 5=password removal, 6=move to trash, 7=remote OCR, 8=apply AI suggestions). A consumption trigger requires filter_filename, filter_path, or filter_mailrule. Actions can modify metadata, permissions, send email or webhooks, remove passwords, or move documents to trash; this affects future documents.", Parameters: dataParameter, Required: []string{"data"}},
		{Name: "paperless_update_workflow", Description: "Update Paperless-ngx automation using nested triggers and actions in data; they are not reusable standalone resources. Use after get_workflow. Trigger/action payloads can grant or revoke document permissions, send email or webhooks, remove passwords, or move future documents to trash.", Parameters: mergeParameters(idParameter, dataParameter), Required: []string{"id", "data"}},
		{Name: "paperless_delete_workflow", Description: "Permanently delete a Paperless-ngx workflow and stop its future automated processing. Use only after get_workflow confirms it should be removed.", Parameters: idParameter, Required: []string{"id"}},
	}
}

func resourceTools(resource configurationResource) []mcp.ToolDefinition {
	return []mcp.ToolDefinition{
		{Name: mcp.ToolName("paperless_list_" + resource.plural), Description: "List Paperless-ngx " + resource.plural + " used to " + resource.purpose + ". Start here to discover available " + resource.plural + ".", Parameters: pageParameters},
		{Name: mcp.ToolName("paperless_get_" + resource.singular), Description: "Get a specific Paperless-ngx " + resource.singular + ". Use after list_" + resource.plural + ".", Parameters: idParameter, Required: []string{"id"}},
		{Name: mcp.ToolName("paperless_create_" + resource.singular), Description: "Create a Paperless-ngx " + resource.singular + " to " + resource.purpose + ". Send documented API fields in data; this creates real configuration. " + resource.danger, Parameters: dataParameter, Required: []string{"data"}},
		{Name: mcp.ToolName("paperless_update_" + resource.singular), Description: "Update a Paperless-ngx " + resource.singular + ". Use after get_" + resource.singular + " and send documented API fields in data. " + resource.danger, Parameters: mergeParameters(idParameter, dataParameter), Required: []string{"id", "data"}},
		{Name: mcp.ToolName("paperless_delete_" + resource.singular), Description: "Permanently delete a Paperless-ngx " + resource.singular + ". Use only after get_" + resource.singular + " confirms it should be removed. " + resource.danger, Parameters: idParameter, Required: []string{"id"}},
	}
}

func mergeParameters(left, right map[string]string) map[string]string {
	result := make(map[string]string, len(left)+len(right))
	maps.Copy(result, left)
	maps.Copy(result, right)
	return result
}

var dispatch = buildDispatch()

func buildDispatch() map[mcp.ToolName]handlerFunc {
	d := map[mcp.ToolName]handlerFunc{
		"paperless_list_documents": listDocuments, "paperless_search_documents": searchDocuments, "paperless_create_document": createDocument, "paperless_get_document": getDocument, "paperless_get_document_ocr_text": getDocumentOCRText, "paperless_update_document": updateDocument, "paperless_delete_document": deleteDocument, "paperless_download_document": downloadDocument,
		"paperless_list_tasks": listResource("/api/tasks/"), "paperless_get_task": getResource("/api/tasks/"), "paperless_get_task_summary": getTaskSummary, "paperless_get_task_status_counts": listMetadata("/api/tasks/status_counts/"), "paperless_list_active_tasks": listMetadata("/api/tasks/active/"), "paperless_run_task": runTask, "paperless_acknowledge_tasks": acknowledgeTasks,
		"paperless_bulk_edit_documents": bulkEditDocuments, "paperless_bulk_edit_objects": bulkEditObjects,
	}
	resources := append([]configurationResource{{singular: "workflow", plural: "workflows"}}, configurationResources...)
	for _, resource := range resources {
		path := "/api/" + resource.plural + "/"
		d[mcp.ToolName("paperless_list_"+resource.plural)] = listResource(path)
		d[mcp.ToolName("paperless_get_"+resource.singular)] = getResource(path)
		d[mcp.ToolName("paperless_create_"+resource.singular)] = createResource(path)
		d[mcp.ToolName("paperless_update_"+resource.singular)] = updateResource(path)
		d[mcp.ToolName("paperless_delete_"+resource.singular)] = deleteResource(path)
	}
	return d
}
