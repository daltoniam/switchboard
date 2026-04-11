package notion

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Data Sources ---
	{
		Name:        mcp.ToolName("notion_create_database"),
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
		Name:        mcp.ToolName("notion_retrieve_data_source"),
		Description: "Retrieve a data source's property schema. Use before query_data_source to understand available columns, types, and filter options.",
		Parameters: map[string]string{
			"data_source_id": "Block ID of the data source (the id field from search results — NOT collection_id)",
		},
		Required: []string{"data_source_id"},
	},
	{
		Name:        mcp.ToolName("notion_update_data_source"),
		Description: "Update a data source's title or property schema",
		Parameters: map[string]string{
			"data_source_id": "Block ID of the data source (the id field from search results — NOT collection_id)",
			"title":          "New title (rich text array)",
			"properties":     "Updated property schema object",
		},
		Required: []string{"data_source_id"},
	},
	{
		Name:        mcp.ToolName("notion_query_data_source"),
		Description: "Query a data source (database) with optional filters and sorts, returning paginated rows. Use retrieve_data_source first to see the schema.",
		Parameters: map[string]string{
			"data_source_id": "Block ID of the data source (the id field from search results — NOT collection_id). The handler resolves the collection internally.",
			"filter":         "Filter object to narrow results. Use retrieve_data_source to see available property names and types for building filters.",
			"sorts":          "Array of sort objects (property + direction)",
			"start_cursor":   "Cursor for pagination",
			"page_size":      "Number of results per page (max 100)",
		},
		Required: []string{"data_source_id"},
	},
	{
		Name:        mcp.ToolName("notion_list_data_source_templates"),
		Description: "List available templates for a data source",
		Parameters: map[string]string{
			"data_source_id": "Block ID of the data source (the id field from search results — NOT collection_id)",
		},
		Required: []string{"data_source_id"},
	},

	// --- Databases ---
	{
		Name:        mcp.ToolName("notion_retrieve_database"),
		Description: "Retrieve a database by block ID. Equivalent to retrieve_data_source — both accept the block ID.",
		Parameters: map[string]string{
			"database_id": "ID of the database",
		},
		Required: []string{"database_id"},
	},

	// --- Pages ---
	{
		Name:        mcp.ToolName("notion_create_page"),
		Description: "Create a new page or database row with properties only (no content blocks). page_id parent creates a subpage; database_id parent creates a row. For pages with content, prefer create_page_with_content.",
		Parameters: map[string]string{
			"parent":     `Parent: {"page_id": "..."} for subpage, or {"database_id": "<collection_id>"} for database row. Use collection_id from search results, NOT the search result id field`,
			"properties": "Page property values object",
			"title":      "Page title (convenience — sets the title property)",
		},
		Required: []string{"parent"},
	},
	{
		Name:        mcp.ToolName("notion_retrieve_page"),
		Description: "Retrieve a page's metadata and properties only. For full page content, prefer get_page_content.",
		Parameters: map[string]string{
			"page_id": "ID of the page",
		},
		Required: []string{"page_id"},
	},
	{
		Name:        mcp.ToolName("notion_update_page"),
		Description: "Update a page's property values (status, assignee, dates, etc). Does not modify page content blocks — use append_block_children or update_block for that.",
		Parameters: map[string]string{
			"page_id":    "ID of the page to update",
			"properties": "Updated property values object",
			"archived":   "Set to true to archive the page",
		},
		Required: []string{"page_id"},
	},
	{
		Name:        mcp.ToolName("notion_move_page"),
		Description: "Move a page to a new parent page or database",
		Parameters: map[string]string{
			"page_id": "ID of the page to move",
			"parent":  "New parent object with page_id or database_id",
		},
		Required: []string{"page_id", "parent"},
	},
	{
		Name:        mcp.ToolName("notion_retrieve_page_property"),
		Description: "Retrieve a single property value. Rarely needed — retrieve_page returns all properties at once.",
		Parameters: map[string]string{
			"page_id":     "ID of the page",
			"property_id": "ID or name of the property to retrieve",
		},
		Required: []string{"page_id", "property_id"},
	},

	// --- Blocks ---
	{
		Name:        mcp.ToolName("notion_retrieve_block"),
		Description: "Retrieve a single block by ID. For full page content, prefer get_page_content.",
		Parameters: map[string]string{
			"block_id": "ID of the block",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        mcp.ToolName("notion_update_block"),
		Description: "Update a block's content",
		Parameters: map[string]string{
			"block_id":     "ID of the block to update",
			"type_content": "Block type-specific content object",
			"archived":     "Set to true to archive the block",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        mcp.ToolName("notion_delete_block"),
		Description: "Delete a block by ID (marks as not alive)",
		Parameters: map[string]string{
			"block_id": "ID of the block to delete",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        mcp.ToolName("notion_get_block_children"),
		Description: "List immediate child blocks of a block. For full page tree, prefer get_page_content.",
		Parameters: map[string]string{
			"block_id": "ID of the parent block",
		},
		Required: []string{"block_id"},
	},
	{
		Name:        mcp.ToolName("notion_append_block_children"),
		Description: "Append new child blocks to a page or block. Use for adding content to existing pages.",
		Parameters: map[string]string{
			"block_id": "ID of the parent block",
			"children": "Array of v3 block objects: {\"type\": \"text\", \"properties\": {\"title\": [[\"content\"]]}}. Types: text, header, sub_header, sub_sub_header, bulleted_list (unordered), numbered_list (ordered, auto-numbered — do not add manual number/letter prefixes), to_do, quote, callout, code (set language via format: {\"code_language\": \"Python\"}), divider, toggle",
		},
		Required: []string{"block_id", "children"},
	},

	// --- Search ---
	{
		Name:        mcp.ToolName("notion_search"),
		Description: "Search across all pages and data sources in the workspace. Start here for most workflows. For database results: use id (block ID) for retrieve_data_source and query_data_source; use collection_id for creating rows via create_page with database_id parent.",
		Parameters: map[string]string{
			"query":    "Search query text. Searches page titles and content.",
			"type":     "Filter by type: \"page\" or \"data_source\"",
			"limit":    "Maximum number of results (default 20)",
			"sort":     "Sort object with field and direction",
			"filters":  "Additional filter object for v3 search",
			"space_id": "Space ID (auto-filled if not provided)",
		},
	},

	// --- Users ---
	{
		Name:        mcp.ToolName("notion_list_users"),
		Description: "List all users in the workspace",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("notion_retrieve_user"),
		Description: "Retrieve a user by ID",
		Parameters: map[string]string{
			"user_id": "ID of the user",
		},
		Required: []string{"user_id"},
	},
	{
		Name:        mcp.ToolName("notion_get_self"),
		Description: "Retrieve the current authenticated user's ID and settings",
		Parameters:  map[string]string{},
	},

	// --- Comments ---
	{
		Name:        mcp.ToolName("notion_create_comment"),
		Description: "Create a comment on a page or in an existing discussion thread. Provide page_id for a new discussion, or discussion_id to reply to an existing thread.",
		Parameters: map[string]string{
			"page_id":       "ID of the page (required for new discussion threads, omit when replying via discussion_id)",
			"text":          "Plain text content of the comment",
			"discussion_id": "ID of an existing discussion thread (from retrieve_comments). Omit for new discussions — use page_id instead.",
		},
		Required: []string{"text"},
	},
	{
		Name:        mcp.ToolName("notion_retrieve_comments"),
		Description: "Retrieve all comment threads on a page. Returns discussions with their comments.",
		Parameters: map[string]string{
			"block_id": "ID of the block or page",
		},
		Required: []string{"block_id"},
	},

	// --- Convenience ---
	{
		Name:        mcp.ToolName("notion_get_page_content"),
		Description: "Retrieve a page and all its block content in one call. Preferred over retrieve_page — returns the full page tree, not just metadata.",
		Parameters: map[string]string{
			"page_id": "ID of the page (from search results or a known page URL)",
			"limit":   "Maximum number of blocks to load (default 100)",
		},
		Required: []string{"page_id"},
	},
	{
		Name:        mcp.ToolName("notion_create_page_with_content"),
		Description: "Create a page or database row with properties and block content in a single atomic transaction. Preferred over create_page + append_block_children — fewer calls, atomic. page_id parent creates a subpage; database_id parent creates a row.",
		Parameters: map[string]string{
			"parent":     `Parent: {"page_id": "..."} for subpage, or {"database_id": "<collection_id>"} for database row. Use collection_id from search results, NOT the search result id field`,
			"properties": "Page property values object",
			"title":      "Page title (convenience)",
			"children":   "Array of v3 block objects: {\"type\": \"text\", \"properties\": {\"title\": [[\"content\"]]}}. Types: text, header, sub_header, sub_sub_header, bulleted_list (unordered), numbered_list (ordered, auto-numbered — do not add manual number/letter prefixes), to_do, quote, callout, code (set language via format: {\"code_language\": \"Python\"}), divider, toggle",
		},
		Required: []string{"parent", "children"},
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	// Data Sources
	mcp.ToolName("notion_create_database"):            createDatabase,
	mcp.ToolName("notion_retrieve_data_source"):       retrieveDataSource,
	mcp.ToolName("notion_update_data_source"):         updateDataSource,
	mcp.ToolName("notion_query_data_source"):          queryDataSource,
	mcp.ToolName("notion_list_data_source_templates"): listDataSourceTemplates,

	// Databases
	mcp.ToolName("notion_retrieve_database"): retrieveDatabase,

	// Pages
	mcp.ToolName("notion_create_page"):            createPage,
	mcp.ToolName("notion_retrieve_page"):          retrievePage,
	mcp.ToolName("notion_update_page"):            updatePage,
	mcp.ToolName("notion_move_page"):              movePage,
	mcp.ToolName("notion_retrieve_page_property"): retrievePageProperty,

	// Blocks
	mcp.ToolName("notion_retrieve_block"):        retrieveBlock,
	mcp.ToolName("notion_update_block"):          updateBlock,
	mcp.ToolName("notion_delete_block"):          deleteBlock,
	mcp.ToolName("notion_get_block_children"):    getBlockChildren,
	mcp.ToolName("notion_append_block_children"): appendBlockChildren,

	// Search
	mcp.ToolName("notion_search"): searchNotion,

	// Users
	mcp.ToolName("notion_list_users"):    listUsers,
	mcp.ToolName("notion_retrieve_user"): retrieveUser,
	mcp.ToolName("notion_get_self"):      getSelf,

	// Comments
	mcp.ToolName("notion_create_comment"):    createComment,
	mcp.ToolName("notion_retrieve_comments"): retrieveComments,

	// Convenience
	mcp.ToolName("notion_get_page_content"):         getPageContent,
	mcp.ToolName("notion_create_page_with_content"): createPageWithContent,
}
