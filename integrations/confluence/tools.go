package confluence

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Spaces ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("confluence_list_spaces"), Description: "List all accessible Confluence spaces. Start here to find space IDs for other operations",
		Parameters: map[string]string{"cursor": "Pagination cursor from previous response", "limit": "Max results per page (default 25, max 250)", "type": "Filter by type: global, personal", "status": "Filter by status: current, archived"},
	},
	{
		Name: mcp.ToolName("confluence_get_space"), Description: "Get details of a specific space by ID",
		Parameters: map[string]string{"space_id": "Space ID"},
		Required:   []string{"space_id"},
	},
	{
		Name: mcp.ToolName("confluence_search"), Description: "Search Confluence content using CQL (Confluence Query Language). Supports pages, blog posts, comments, and attachments",
		Parameters: map[string]string{"cql": "CQL query (e.g., 'type=page AND space=DEV AND title~\"design doc\"')", "limit": "Max results per page (default 25, max 100)", "start": "Pagination offset (0-based)", "excerpt": "Include excerpt in results: none, highlight, indexed (default none)"},
		Required:   []string{"cql"},
	},

	// ── Pages ───────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("confluence_list_pages"), Description: "List pages, optionally filtered by space or title",
		Parameters: map[string]string{"space_id": "Filter by space ID", "title": "Filter by exact title", "status": "Filter by status: current, trashed, draft (default current)", "cursor": "Pagination cursor from previous response", "limit": "Max results per page (default 25, max 250)", "sort": "Sort order: id, -id, title, -title, created-date, -created-date, modified-date, -modified-date"},
	},
	{
		Name: mcp.ToolName("confluence_get_page"), Description: "Get full details of a specific page by ID, including body content",
		Parameters: map[string]string{"page_id": "Page ID", "body_format": "Body format to return: storage, atlas_doc_format, view (default storage)", "version": "Specific version number to retrieve"},
		Required:   []string{"page_id"},
	},
	{
		Name: mcp.ToolName("confluence_create_page"), Description: "Create a new page in a space",
		Parameters: map[string]string{"space_id": "Space ID to create page in", "title": "Page title", "body_value": "Page body content", "body_format": "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)", "parent_id": "Parent page ID for nesting", "status": "Page status: current, draft (default current)"},
		Required:   []string{"space_id", "title", "body_value"},
	},
	{
		Name: mcp.ToolName("confluence_update_page"), Description: "Update an existing page. Requires the current version number (use confluence_get_page to find it)",
		Parameters: map[string]string{"page_id": "Page ID", "title": "New page title", "body_value": "New page body content", "body_format": "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)", "version_number": "New version number (must be current version + 1)", "version_message": "Version change message", "status": "Page status: current, draft"},
		Required:   []string{"page_id", "title", "body_value", "version_number"},
	},
	{
		Name: mcp.ToolName("confluence_delete_page"), Description: "Delete a page by ID",
		Parameters: map[string]string{"page_id": "Page ID"},
		Required:   []string{"page_id"},
	},
	{
		Name: mcp.ToolName("confluence_get_page_children"), Description: "Get child pages of a specific page",
		Parameters: map[string]string{"page_id": "Parent page ID", "cursor": "Pagination cursor from previous response", "limit": "Max results per page (default 25, max 250)", "sort": "Sort order: id, -id, title, -title, created-date, -created-date, modified-date, -modified-date"},
		Required:   []string{"page_id"},
	},

	// ── Blog Posts ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("confluence_list_blog_posts"), Description: "List blog posts, optionally filtered by space or title",
		Parameters: map[string]string{"space_id": "Filter by space ID", "title": "Filter by exact title", "status": "Filter by status: current, trashed, draft (default current)", "cursor": "Pagination cursor from previous response", "limit": "Max results per page (default 25, max 250)", "sort": "Sort order: id, -id, title, -title, created-date, -created-date, modified-date, -modified-date"},
	},
	{
		Name: mcp.ToolName("confluence_get_blog_post"), Description: "Get full details of a specific blog post by ID, including body content",
		Parameters: map[string]string{"blogpost_id": "Blog post ID", "body_format": "Body format to return: storage, atlas_doc_format, view (default storage)", "version": "Specific version number to retrieve"},
		Required:   []string{"blogpost_id"},
	},
	{
		Name: mcp.ToolName("confluence_create_blog_post"), Description: "Create a new blog post in a space",
		Parameters: map[string]string{"space_id": "Space ID to create blog post in", "title": "Blog post title", "body_value": "Blog post body content", "body_format": "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)", "status": "Blog post status: current, draft (default current)"},
		Required:   []string{"space_id", "title", "body_value"},
	},
	{
		Name: mcp.ToolName("confluence_update_blog_post"), Description: "Update an existing blog post. Requires the current version number (use confluence_get_blog_post to find it)",
		Parameters: map[string]string{"blogpost_id": "Blog post ID", "title": "New blog post title", "body_value": "New blog post body content", "body_format": "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)", "version_number": "New version number (must be current version + 1)", "version_message": "Version change message", "status": "Blog post status: current, draft"},
		Required:   []string{"blogpost_id", "title", "body_value", "version_number"},
	},
	{
		Name: mcp.ToolName("confluence_delete_blog_post"), Description: "Delete a blog post by ID",
		Parameters: map[string]string{"blogpost_id": "Blog post ID"},
		Required:   []string{"blogpost_id"},
	},

	// ── Comments ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("confluence_list_comments"), Description: "List footer comments on a page or blog post",
		Parameters: map[string]string{"parent_type": "Parent content type: page, blogpost", "parent_id": "Parent content ID", "cursor": "Pagination cursor from previous response", "limit": "Max results per page (default 25, max 250)"},
		Required:   []string{"parent_type", "parent_id"},
	},
	{
		Name: mcp.ToolName("confluence_create_comment"), Description: "Add a footer comment to a page or blog post",
		Parameters: map[string]string{"parent_type": "Parent content type: page, blogpost", "parent_id": "Parent content ID", "body_value": "Comment body content", "body_format": "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)"},
		Required:   []string{"parent_type", "parent_id", "body_value"},
	},
	{
		Name: mcp.ToolName("confluence_delete_comment"), Description: "Delete a footer comment by ID",
		Parameters: map[string]string{"comment_id": "Comment ID"},
		Required:   []string{"comment_id"},
	},
}
