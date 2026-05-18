package metabase

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Databases ---
	{
		Name:        mcp.ToolName("metabase_list_databases"),
		Description: "List all databases configured in Metabase",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("metabase_get_database"),
		Description: "Get details of a specific database including its tables",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_list_tables"),
		Description: "List all tables in a specific database with metadata",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_get_table"),
		Description: "Get detailed metadata for a specific table",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table_id"), Description: "Table ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_get_table_fields"),
		Description: "Get all fields/columns for a specific table with types and metadata",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("table_id"), Description: "Table ID", Required: true}},
	},

	// --- Queries ---
	{
		Name:        mcp.ToolName("metabase_execute_query"),
		Description: "Execute a native SQL analytics query against a Metabase-connected database and return results as JSON. Use for ad-hoc analytics, BI reporting, and data exploration.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("database_id"), Description: "Database ID to query", Required: true}, {Name: mcp.ParamName("query"), Description: "SQL query string", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_execute_card"),
		Description: "Execute a saved question/card and return its results",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("card_id"), Description: "Card/question ID", Required: true}, {Name: mcp.ParamName("parameters"), Description: "Optional JSON array of parameter objects [{type, target, value}]"}},
	},

	// --- Cards (Saved Questions) ---
	{
		Name:        mcp.ToolName("metabase_list_cards"),
		Description: "List all saved questions/cards. Optionally filter by type.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("filter"), Description: "Filter: all (default), mine, bookmarked, archived"}},
	},
	{
		Name:        mcp.ToolName("metabase_get_card"),
		Description: "Get details of a specific saved question/card",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("card_id"), Description: "Card ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_create_card"),
		Description: "Create a new saved question/card with a native SQL query",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Name of the question", Required: true}, {Name: mcp.ParamName("database_id"), Description: "Database ID", Required: true}, {Name: mcp.ParamName("query"), Description: "SQL query string", Required: true}, {Name: mcp.ParamName("description"), Description: "Optional description"}, {Name: mcp.ParamName("collection_id"), Description: "Optional collection ID to save into"}, {Name: mcp.ParamName("display"), Description: "Visualization type: table, bar, line, pie, scalar, etc. (default: table)"}},
	},
	{
		Name:        mcp.ToolName("metabase_update_card"),
		Description: "Update a saved question/card (name, description, query, visualization)",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("card_id"), Description: "Card ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("query"), Description: "New SQL query"}, {Name: mcp.ParamName("database_id"), Description: "Database ID (required if changing query)"}, {Name: mcp.ParamName("display"), Description: "Visualization type: table, bar, line, pie, scalar, etc."}, {Name: mcp.ParamName("archived"), Description: "Set to true to archive the card"}},
	},
	{
		Name:        mcp.ToolName("metabase_delete_card"),
		Description: "Delete (archive) a saved question/card",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("card_id"), Description: "Card ID", Required: true}},
	},

	// --- Dashboards ---
	{
		Name:        mcp.ToolName("metabase_list_dashboards"),
		Description: "List all Metabase analytics dashboards for reporting and data visualization",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("metabase_get_dashboard"),
		Description: "Get details of a dashboard including its cards and layout",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("dashboard_id"), Description: "Dashboard ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_create_dashboard"),
		Description: "Create a new dashboard",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Dashboard name", Required: true}, {Name: mcp.ParamName("description"), Description: "Optional description"}, {Name: mcp.ParamName("collection_id"), Description: "Optional collection ID"}},
	},
	{
		Name:        mcp.ToolName("metabase_update_dashboard"),
		Description: "Update a dashboard's name, description, or other properties",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("dashboard_id"), Description: "Dashboard ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("archived"), Description: "Set to true to archive the dashboard"}},
	},
	{
		Name:        mcp.ToolName("metabase_delete_dashboard"),
		Description: "Delete (archive) a dashboard",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("dashboard_id"), Description: "Dashboard ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_add_card_to_dashboard"),
		Description: "Add a saved question/card to a dashboard",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("dashboard_id"), Description: "Dashboard ID", Required: true}, {Name: mcp.ParamName("card_id"), Description: "Card ID to add", Required: true}, {Name: mcp.ParamName("size_x"), Description: "Width in grid units (default: 6)"}, {Name: mcp.ParamName("size_y"), Description: "Height in grid units (default: 4)"}, {Name: mcp.ParamName("row"), Description:

		// --- Collections ---
		"Row position (default: 0)"}, {Name: mcp.ParamName("col"), Description: "Column position (default: 0)"}},
	},

	{
		Name:        mcp.ToolName("metabase_list_collections"),
		Description: "List all collections (folders for organizing questions and dashboards)",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("metabase_get_collection"),
		Description: "Get details and items in a specific collection",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("collection_id"), Description: "Collection ID (use 'root' for the root collection)", Required: true}},
	},
	{
		Name:        mcp.ToolName("metabase_create_collection"),
		Description: "Create a new collection for organizing questions and dashboards",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Collection name", Required: true}, {Name: mcp.ParamName("description"), Description: "Optional description"}, {Name: mcp.ParamName("parent_id"), Description: "Optional parent collection ID for nesting"}},
	},
	{
		Name:        mcp.ToolName("metabase_update_collection"),
		Description: "Update a collection's name, description, or parent",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("collection_id"), Description: "Collection ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("parent_id"), Description: "New parent collection ID"},

		// --- Search ---
		{Name: mcp.ParamName("archived"), Description: "Set to true to archive"}},
	},

	{
		Name:        mcp.ToolName("metabase_search"),
		Description: "Search across all Metabase content (questions, dashboards, collections, tables, databases). Start here for BI and reporting workflows.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Search query string", Required: true}, {Name: mcp.ParamName("models"), Description: "Comma-separated types to search: card, dashboard, collection, table, database"}},
	},
}

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
