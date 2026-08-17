package projectinterop

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "projectinterop_list_projects",
		Description: "List project-interop projects with their repository and default branch",
		Parameters:  map[string]string{},
	},
	{
		Name:        "projectinterop_get_project",
		Description: "Get a fully merged project-interop project definition, including its repo-local override",
		Parameters: map[string]string{
			"name": "Project name",
		},
		Required: []string{"name"},
	},
	{
		Name:        "projectinterop_create_project",
		Description: "Create a project-interop project definition in the user-level store",
		Parameters: map[string]string{
			"name":   "Project name",
			"repo":   "Path to the source repository (optional)",
			"branch": "Default branch (optional)",
		},
		Required: []string{"name"},
	},
	{
		Name:        "projectinterop_update_project",
		Description: "Apply an RFC 7396 JSON merge patch to a user-level project-interop definition",
		Parameters: map[string]string{
			"name":  "Project name",
			"patch": "JSON merge patch object; repo-local overrides are never modified",
		},
		Required: []string{"name", "patch"},
	},
	{
		Name:        "projectinterop_delete_project",
		Description: "Delete a project-interop definition from the user-level store without deleting context or repo-local files",
		Parameters: map[string]string{
			"name": "Project name",
		},
		Required: []string{"name"},
	},
	{
		Name:        "projectinterop_get_context",
		Description: "List, search, or retrieve assembled context for a project-interop project",
		Parameters: map[string]string{
			"name":  "Project name",
			"query": "Case-insensitive path filter; omit to list the context manifest",
			"path":  "Exact context path to retrieve full content",
			"role":  "Agent role whose context overrides should be applied",
		},
		Required: []string{"name"},
	},
}
