package confluence

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Spaces ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("confluence_list_spaces"), Description: "List all accessible Confluence spaces. Start here to find space IDs for other operations",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}, {Name: mcp.ParamName("limit"), Description: "Max results per page (default 25, max 250)"}, {Name: mcp.ParamName("type"), Description: "Filter by type: global, personal"}, {Name: mcp.ParamName("status"), Description: "Filter by status: current, archived"}},
	},
	{
		Name: mcp.ToolName("confluence_get_space"), Description: "Get details of a specific space by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("space_id"), Description: "Space ID", Required: true}},
	},
	{
		Name: mcp.ToolName("confluence_search"), Description: "Search Confluence content using CQL (Confluence Query Language). Supports pages, blog posts, comments, and attachments",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("cql"), Description: `CQL query (e.g., 'type=page AND space=DEV AND title~"design doc"')`, Required: true}, {Name: mcp.ParamName("limit"), Description: "Max results per page (default 25, max 100)"}, {Name: mcp.ParamName("start"), Description: "Pagination offset (0-based)"}, {Name: mcp.ParamName(

		// ── Pages ───────────────────────────────────────────────────────
		"excerpt"), Description: "Include excerpt in results: none, highlight, indexed (default none)"}},
	},

	{
		Name: mcp.ToolName("confluence_list_pages"), Description: "List pages, optionally filtered by space or title",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("space_id"), Description: "Filter by space ID"}, {Name: mcp.ParamName("title"), Description: "Filter by exact title"}, {Name: mcp.ParamName("status"), Description: "Filter by status: current, trashed, draft (default current)"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}, {Name: mcp.ParamName("limit"), Description: "Max results per page (default 25, max 250)"}, {Name: mcp.ParamName("sort"), Description: "Sort order: id, -id, title, -title, created-date, -created-date, modified-date, -modified-date"}},
	},
	{
		Name: mcp.ToolName("confluence_get_page"), Description: "Get full details of a specific page by ID, including body content",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Page ID", Required: true}, {Name: mcp.ParamName("body_format"), Description: "Body format to return: storage, atlas_doc_format, view (default storage)"}, {Name: mcp.ParamName("version"), Description: "Specific version number to retrieve"}},
	},
	{
		Name: mcp.ToolName("confluence_create_page"), Description: "Create a new page in a space",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("space_id"), Description: "Space ID to create page in", Required: true}, {Name: mcp.ParamName("title"), Description: "Page title", Required: true}, {Name: mcp.ParamName("body_value"), Description: "Page body content", Required: true}, {Name: mcp.ParamName("body_format"), Description: "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)"}, {Name: mcp.ParamName("parent_id"), Description: "Parent page ID for nesting"}, {Name: mcp.ParamName("status"), Description: "Page status: current, draft (default current)"}},
	},
	{
		Name: mcp.ToolName("confluence_update_page"), Description: "Update an existing page. Requires the current version number (use confluence_get_page to find it)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Page ID", Required: true}, {Name: mcp.ParamName("title"), Description: "New page title", Required: true}, {Name: mcp.ParamName("body_value"), Description: "New page body content", Required: true}, {Name: mcp.ParamName("body_format"), Description: "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)"}, {Name: mcp.ParamName("version_number"), Description: "New version number (must be current version + 1)", Required: true}, {Name: mcp.ParamName("version_message"), Description: "Version change message"}, {Name: mcp.ParamName("status"), Description: "Page status: current, draft"}},
	},
	{
		Name: mcp.ToolName("confluence_delete_page"), Description: "Delete a page by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Page ID", Required: true}},
	},
	{
		Name: mcp.ToolName("confluence_get_page_children"), Description: "Get child pages of a specific page",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("page_id"), Description: "Parent page ID", Required: true}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}, {Name: mcp.ParamName("limit"), Description: "Max results per page (default 25, max 250)"}, {Name: mcp.ParamName("sort"), Description: "Sort order: id, -id, title, -title, created-date, -created-date, modified-date, -modified-date"}},
	},

	// ── Blog Posts ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("confluence_list_blog_posts"), Description: "List blog posts, optionally filtered by space or title",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("space_id"), Description: "Filter by space ID"}, {Name: mcp.ParamName("title"), Description: "Filter by exact title"}, {Name: mcp.ParamName("status"), Description: "Filter by status: current, trashed, draft (default current)"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}, {Name: mcp.ParamName("limit"), Description: "Max results per page (default 25, max 250)"}, {Name: mcp.ParamName("sort"), Description: "Sort order: id, -id, title, -title, created-date, -created-date, modified-date, -modified-date"}},
	},
	{
		Name: mcp.ToolName("confluence_get_blog_post"), Description: "Get full details of a specific blog post by ID, including body content",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("blogpost_id"), Description: "Blog post ID", Required: true}, {Name: mcp.ParamName("body_format"), Description: "Body format to return: storage, atlas_doc_format, view (default storage)"}, {Name: mcp.ParamName("version"), Description: "Specific version number to retrieve"}},
	},
	{
		Name: mcp.ToolName("confluence_create_blog_post"), Description: "Create a new blog post in a space",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("space_id"), Description: "Space ID to create blog post in", Required: true}, {Name: mcp.ParamName("title"), Description: "Blog post title", Required: true}, {Name: mcp.ParamName("body_value"), Description: "Blog post body content", Required: true}, {Name: mcp.ParamName("body_format"), Description: "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)"}, {Name: mcp.ParamName("status"), Description: "Blog post status: current, draft (default current)"}},
	},
	{
		Name: mcp.ToolName("confluence_update_blog_post"), Description: "Update an existing blog post. Requires the current version number (use confluence_get_blog_post to find it)",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("blogpost_id"), Description: "Blog post ID", Required: true}, {Name: mcp.ParamName("title"), Description: "New blog post title", Required: true}, {Name: mcp.ParamName("body_value"), Description: "New blog post body content", Required: true}, {Name: mcp.ParamName("body_format"), Description: "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)"}, {Name: mcp.ParamName("version_number"), Description: "New version number (must be current version + 1)", Required: true}, {Name: mcp.ParamName("version_message"), Description: "Version change message"}, {Name: mcp.ParamName("status"), Description: "Blog post status: current, draft"}},
	},
	{
		Name: mcp.ToolName("confluence_delete_blog_post"), Description: "Delete a blog post by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("blogpost_id"), Description: "Blog post ID", Required: true}},
	},

	// ── Comments ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("confluence_list_comments"), Description: "List footer comments on a page or blog post",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("parent_type"), Description: "Parent content type: page, blogpost", Required: true}, {Name: mcp.ParamName("parent_id"), Description: "Parent content ID", Required: true}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}, {Name: mcp.ParamName("limit"), Description: "Max results per page (default 25, max 250)"}},
	},
	{
		Name: mcp.ToolName("confluence_create_comment"), Description: "Add a footer comment to a page or blog post",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("parent_type"), Description: "Parent content type: page, blogpost", Required: true}, {Name: mcp.ParamName("parent_id"), Description: "Parent content ID", Required: true}, {Name: mcp.ParamName("body_value"), Description: "Comment body content", Required: true}, {Name: mcp.ParamName("body_format"), Description: "Body format: storage (XHTML), atlas_doc_format (ADF JSON) (default storage)"}},
	},
	{
		Name: mcp.ToolName("confluence_delete_comment"), Description: "Delete a footer comment by ID",
		Parameters: []mcp.Parameter{{Name: mcp.ParamName("comment_id"), Description: "Comment ID", Required: true}},
	},
}
