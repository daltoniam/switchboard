package openproject

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: "openproject_list_work_packages", Access: mcp.AccessRead,
		Description: "List or search OpenProject Community work packages, tasks and issues in the configured project, including open and closed status. Start here to find a work package before openproject_get_work_package. Fetches one upstream page, sorted by numeric ID. page is a one-based PAGE NUMBER, not a record offset; server page size may be smaller than requested. Other projects and descendants are omitted and counted without changing upstream continuation. No writes, users, comments or attachments.",
		Parameters: map[string]string{
			"subject_or_id": "Optional subject or work package ID search text, at most 256 characters; encoded as the documented subjectOrId filter, not a raw query",
			"page":          "One-based upstream page number, positive integer (default 1); concurrent changes can shift pages",
			"limit":         "Requested upstream page size, integer 1-50 (default 20); use a smaller size if the 40 KiB output bound is exceeded",
		},
	},
	{
		Name: "openproject_get_work_package", Access: mcp.AccessRead,
		Description: "Get one exact OpenProject Community work package, task or issue from the configured project. Use after openproject_list_work_packages with its positive numeric id, not displayId. Returns status and type labels, dates and untrusted description text capped at 8,000 characters with a truncation flag. A not-found response can mean not visible; project mismatch fails closed. No HTML, custom fields, users, comments, files or writes.",
		Parameters: map[string]string{
			"id": "Required positive numeric work package id from discovery; not a semantic displayId, URL or path",
		},
		Required: []string{"id"},
	},
}
