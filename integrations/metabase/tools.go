package metabase

import (
	_ "embed"

	mcp "github.com/daltoniam/switchboard"
)

//go:embed tools.yaml
var toolsYAML []byte

var tools = mcp.MustLoadToolsYAML(toolsYAML)

var dispatch = map[mcp.ToolName]handlerFunc{
	// Databases
	mcp.ToolName("metabase_list_databases"):   listDatabases,
	mcp.ToolName("metabase_get_database"):     getDatabase,
	mcp.ToolName("metabase_list_tables"):      listTables,
	mcp.ToolName("metabase_get_table"):        getTable,
	mcp.ToolName("metabase_get_table_fields"): getTableFields,

	// Queries
	mcp.ToolName("metabase_execute_query"): executeQuery,
	mcp.ToolName("metabase_execute_card"):  executeCard,

	// Cards
	mcp.ToolName("metabase_list_cards"):  listCards,
	mcp.ToolName("metabase_get_card"):    getCard,
	mcp.ToolName("metabase_create_card"): createCard,
	mcp.ToolName("metabase_update_card"): updateCard,
	mcp.ToolName("metabase_delete_card"): deleteCard,

	// Dashboards
	mcp.ToolName("metabase_list_dashboards"):       listDashboards,
	mcp.ToolName("metabase_get_dashboard"):         getDashboard,
	mcp.ToolName("metabase_create_dashboard"):      createDashboard,
	mcp.ToolName("metabase_update_dashboard"):      updateDashboard,
	mcp.ToolName("metabase_delete_dashboard"):      deleteDashboard,
	mcp.ToolName("metabase_add_card_to_dashboard"): addCardToDashboard,

	// Collections
	mcp.ToolName("metabase_list_collections"):  listCollections,
	mcp.ToolName("metabase_get_collection"):    getCollection,
	mcp.ToolName("metabase_create_collection"): createCollection,
	mcp.ToolName("metabase_update_collection"): updateCollection,

	// Search
	mcp.ToolName("metabase_search"): search,
}
