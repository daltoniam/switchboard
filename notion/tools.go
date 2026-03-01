package notion

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Data Sources ---
	{
		Name:        "notion_create_database",
		Description: "Create a new database under a parent page",
		Parameters: map[string]string{
			"parent":     "Parent object with page_id (e.g. {\"page_id\": \"...\"})",
			"title":      "Title of the database (rich text array)",
			"properties": "Property schema object defining columns and their types",
			"is_inline":  "Set to true to create an inline database (default false)",
		},
		Required: []string{"parent"},
	},
	{
		Name:        "notion_retrieve_data_source",
		Description: "Retrieve a data source by ID, including its property schema",
		Parameters: map[string]string{
			"data_source_id": "ID of the data source",
		},
		Required: []string{"data_source_id"},
	},
	{
		Name:        "notion_update_data_source",
		Description: "Update a data source's title or property schema (description updates require the Update Database API)",
		Parameters: map[string]string{
			"data_source_id": "ID of the data source",
			"title":          "New title (rich text array)",
			"properties":     "Updated property schema object",
		},
		Required: []string{"data_source_id"},
	},
	{
		Name:        "notion_query_data_source",
		Description: "Query a data source with optional filters and sorts, returning paginated rows",
		Parameters: map[string]string{
			"data_source_id": "ID of the data source to query",
			"filter":         "Filter object to narrow results",
			"sorts":          "Array of sort objects (property + direction)",
			"start_cursor":   "Cursor for pagination",
			"page_size":      "Number of results per page (max 100)",
		},
		Required: []string{"data_source_id"},
	},
	{
		Name:        "notion_list_data_source_templates",
		Description: "List available templates for a data source",
		Parameters: map[string]string{
			"data_source_id": "ID of the data source",
			"start_cursor":   "Cursor for pagination",
			"page_size":      "Number of results per page (max 100)",
		},
		Required: []string{"data_source_id"},
	},

	// --- Databases ---
	{
		Name:        "notion_retrieve_database",
		Description: "Retrieve a database by ID, including its property schema and metadata",
		Parameters: map[string]string{
			"database_id": "ID of the database",
		},
		Required: []string{"database_id"},
	},

	// --- Pages ---
	{
		Name:        "notion_create_page",
		Description: "Create a new page within a parent page or database",
		Parameters: map[string]string{
			"parent":     "Parent object with page_id or database_id",
			"properties": "Page property values object",
			"children":   "Array of block objects for initial page content",
			"icon":       "Icon object (emoji or external URL)",
			"cover":      "Cover image object (external URL)",
		},
		Required: []string{"parent"},
	},
	{
		Name:        "notion_retrieve_page",
		Description: "Retrieve a page by ID, including its property values",
		Parameters: map[string]string{
			"page_id": "ID of the page",
		},
		Required: []string{"page_id"},
	},
	{
		Name:        "notion_update_page",
		Description: "Update a page's properties, icon, cover, or archived status",
		Parameters: map[string]string{
			"page_id":    "ID of the page to update",
			"properties": "Updated property values object",
			"icon":       "Updated icon object",
			"cover":      "Updated cover image object",
			"archived":   "Set to true to archive the page",
		},
		Required: []string{"page_id"},
	},
	{
		Name:        "notion_move_page",
		Description: "Move a page to a new parent page or database",
		Parameters: map[string]string{
			"page_id": "ID of the page to move",
			"parent":  "New parent object with page_id or database_id",
		},
		Required: []string{"page_id", "parent"},
	},
	{
		Name:        "notion_retrieve_page_property",
		Description: "Retrieve a specific property value from a page, with pagination for large values",
		Parameters: map[string]string{
			"page_id":      "ID of the page",
			"property_id":  "ID of the property to retrieve",
			"start_cursor": "Cursor for pagination",
			"page_size":    "Number of results per page (max 100)",
		},
		Required: []string{"page_id", "property_id"},
	},

	// --- Blocks ---
	{
		Name:        "notion_retrieve_block",
		Description: "Retrieve a block by ID",
		Parameters: map[string]string{
			"block_id": "ID of the block",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        "notion_update_block",
		Description: "Update a block's content or archived status",
		Parameters: map[string]string{
			"block_id":      "ID of the block to update",
			"type_content":  "Block type-specific content object (keys merged into request body)",
			"archived":      "Set to true to archive the block",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        "notion_delete_block",
		Description: "Delete (archive) a block by ID",
		Parameters: map[string]string{
			"block_id": "ID of the block to delete",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        "notion_get_block_children",
		Description: "List all child blocks of a given block, with pagination",
		Parameters: map[string]string{
			"block_id":     "ID of the parent block",
			"start_cursor": "Cursor for pagination",
			"page_size":    "Number of results per page (max 100)",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        "notion_append_block_children",
		Description: "Append new child blocks to a parent block",
		Parameters: map[string]string{
			"block_id": "ID of the parent block",
			"children": "Array of block objects to append",
		},
		Required: []string{"block_id", "children"},
	},

	// --- Search ---
	{
		Name:        "notion_search",
		Description: "Search across all pages and databases in the workspace",
		Parameters: map[string]string{
			"query":        "Search query text",
			"filter":       "Filter object to narrow by object type (value: \"page\" or \"data_source\")",
			"sort":         "Sort object (direction + timestamp field)",
			"start_cursor": "Cursor for pagination",
			"page_size":    "Number of results per page (max 100)",
		},
	},

	// --- Users ---
	{
		Name:        "notion_list_users",
		Description: "List all users in the workspace, with pagination",
		Parameters: map[string]string{
			"start_cursor": "Cursor for pagination",
			"page_size":    "Number of results per page (max 100)",
		},
	},
	{
		Name:        "notion_retrieve_user",
		Description: "Retrieve a user by ID",
		Parameters: map[string]string{
			"user_id": "ID of the user",
		},
		Required: []string{"user_id"},
	},
	{
		Name:        "notion_get_self",
		Description: "Retrieve the bot user associated with the current API token",
		Parameters:  map[string]string{},
	},

	// --- Comments ---
	{
		Name:        "notion_create_comment",
		Description: "Create a comment on a page or in an existing discussion thread",
		Parameters: map[string]string{
			"rich_text":     "Rich text array for the comment body",
			"parent":        "Parent object with page_id to start a new discussion",
			"discussion_id": "ID of an existing discussion thread to reply to",
		},
		Required: []string{"rich_text"},
	},
	{
		Name:        "notion_retrieve_comments",
		Description: "Retrieve comments on a block or page, with pagination",
		Parameters: map[string]string{
			"block_id":     "ID of the block or page",
			"start_cursor": "Cursor for pagination",
			"page_size":    "Number of results per page (max 100)",
		},
		Required: []string{"block_id"},
	},

	// --- Convenience ---
	{
		Name:        "notion_get_page_content",
		Description: "Retrieve a page and all its block content in one call, recursively fetching nested blocks",
		Parameters: map[string]string{
			"page_id":   "ID of the page",
			"max_depth": "Maximum depth for recursive block fetching (default 3)",
		},
		Required: []string{"page_id"},
	},
	{
		Name:        "notion_create_page_with_content",
		Description: "Create a page with properties and block content in a single call",
		Parameters: map[string]string{
			"parent":     "Parent object with page_id or database_id",
			"properties": "Page property values object",
			"children":   "Array of block objects for page content",
			"icon":       "Icon object (emoji or external URL)",
			"cover":      "Cover image object (external URL)",
		},
		Required: []string{"parent", "children"},
	},
}

var dispatch = map[string]handlerFunc{
	// Data Sources
	"notion_create_database":            createDatabase,
	"notion_retrieve_data_source":       retrieveDataSource,
	"notion_update_data_source":         updateDataSource,
	"notion_query_data_source":          queryDataSource,
	"notion_list_data_source_templates": listDataSourceTemplates,

	// Databases
	"notion_retrieve_database": retrieveDatabase,

	// Pages
	"notion_create_page":             createPage,
	"notion_retrieve_page":           retrievePage,
	"notion_update_page":             updatePage,
	"notion_move_page":               movePage,
	"notion_retrieve_page_property":  retrievePageProperty,

	// Blocks
	"notion_retrieve_block":        retrieveBlock,
	"notion_update_block":          updateBlock,
	"notion_delete_block":          deleteBlock,
	"notion_get_block_children":    getBlockChildren,
	"notion_append_block_children": appendBlockChildren,

	// Search
	"notion_search": searchNotion,

	// Users
	"notion_list_users":    listUsers,
	"notion_retrieve_user": retrieveUser,
	"notion_get_self":      getSelf,

	// Comments
	"notion_create_comment":    createComment,
	"notion_retrieve_comments": retrieveComments,

	// Convenience
	"notion_get_page_content":          getPageContent,
	"notion_create_page_with_content":  createPageWithContent,
}
