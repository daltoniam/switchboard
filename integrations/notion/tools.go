package notion

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)

var dispatch = map[mcp.ToolName]handlerFunc{

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
