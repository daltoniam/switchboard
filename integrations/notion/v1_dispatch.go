package notion

import mcp "github.com/daltoniam/switchboard"

// dispatchV1 is the v1 backend's tool routing table. Mirrors the v3
// `dispatch` map; same tool names, different handlers. When the
// integration is configured with an OAuth access token, notion.Execute
// looks tools up here.
//
// Tools that have no v1 equivalent (none today, but reserved for future
// v3-only surfaces) should be left out — notion.Execute returns a
// "not supported on v1 backend" error for missing entries.
var dispatchV1 = map[mcp.ToolName]v1HandlerFunc{
	// Data Sources / Databases
	mcp.ToolName("notion_create_database"):            v1CreateDatabase,
	mcp.ToolName("notion_retrieve_data_source"):       v1RetrieveDataSource,
	mcp.ToolName("notion_update_data_source"):         v1UpdateDataSource,
	mcp.ToolName("notion_query_data_source"):          v1QueryDataSource,
	mcp.ToolName("notion_list_data_source_templates"): v1ListDataSourceTemplates,
	mcp.ToolName("notion_retrieve_database"):          v1RetrieveDatabase,

	// Pages
	mcp.ToolName("notion_create_page"):            v1CreatePage,
	mcp.ToolName("notion_retrieve_page"):          v1RetrievePage,
	mcp.ToolName("notion_update_page"):            v1UpdatePage,
	mcp.ToolName("notion_move_page"):              v1MovePage,
	mcp.ToolName("notion_retrieve_page_property"): v1RetrievePageProperty,

	// Blocks
	mcp.ToolName("notion_retrieve_block"):        v1RetrieveBlock,
	mcp.ToolName("notion_update_block"):          v1UpdateBlock,
	mcp.ToolName("notion_delete_block"):          v1DeleteBlock,
	mcp.ToolName("notion_get_block_children"):    v1GetBlockChildren,
	mcp.ToolName("notion_append_block_children"): v1AppendBlockChildren,

	// Search
	mcp.ToolName("notion_search"): v1Search,

	// Users
	mcp.ToolName("notion_list_users"):    v1ListUsers,
	mcp.ToolName("notion_retrieve_user"): v1RetrieveUser,
	mcp.ToolName("notion_get_self"):      v1GetSelf,

	// Comments
	mcp.ToolName("notion_create_comment"):    v1CreateComment,
	mcp.ToolName("notion_retrieve_comments"): v1RetrieveComments,

	// Convenience
	mcp.ToolName("notion_get_page_content"):         v1GetPageContent,
	mcp.ToolName("notion_create_page_with_content"): v1CreatePageWithContent,
}
