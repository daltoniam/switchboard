package metabase

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- Databases ---
	{
		Name:        mcp.ToolName("metabase_list_databases"),
		Description: "List all databases configured in Metabase",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("metabase_get_database"),
		Description: "Get details of a specific database including its tables",
		Parameters:  map[string]string{"database_id": "Database ID"},
		Required:    []string{"database_id"},
	},
	{
		Name:        mcp.ToolName("metabase_list_tables"),
		Description: "List all tables in a specific database with metadata",
		Parameters:  map[string]string{"database_id": "Database ID"},
		Required:    []string{"database_id"},
	},
	{
		Name:        mcp.ToolName("metabase_get_table"),
		Description: "Get detailed metadata for a specific table",
		Parameters:  map[string]string{"table_id": "Table ID"},
		Required:    []string{"table_id"},
	},
	{
		Name:        mcp.ToolName("metabase_get_table_fields"),
		Description: "Get all fields/columns for a specific table with types and metadata",
		Parameters:  map[string]string{"table_id": "Table ID"},
		Required:    []string{"table_id"},
	},

	// --- Queries ---
	{
		Name:        mcp.ToolName("metabase_execute_query"),
		Description: "Execute a native SQL analytics query against a Metabase-connected database and return results as JSON. Use for ad-hoc analytics, BI reporting, and data exploration.",
		Parameters: map[string]string{
			"database_id": "Database ID to query",
			"query":       "SQL query string",
		},
		Required: []string{"database_id", "query"},
	},
	{
		Name:        mcp.ToolName("metabase_execute_card"),
		Description: "Execute a saved question/card and return its results",
		Parameters: map[string]string{
			"card_id":    "Card/question ID",
			"parameters": "Optional JSON array of parameter objects [{type, target, value}]",
		},
		Required: []string{"card_id"},
	},

	// --- Cards (Saved Questions) ---
	{
		Name:        mcp.ToolName("metabase_list_cards"),
		Description: "List all saved questions/cards. Optionally filter by type.",
		Parameters: map[string]string{
			"filter": "Filter: all (default), mine, bookmarked, archived",
		},
	},
	{
		Name:        mcp.ToolName("metabase_get_card"),
		Description: "Get details of a specific saved question/card",
		Parameters:  map[string]string{"card_id": "Card ID"},
		Required:    []string{"card_id"},
	},
	{
		Name:        mcp.ToolName("metabase_create_card"),
		Description: "Create a new saved question/card with a native SQL query",
		Parameters: map[string]string{
			"name":          "Name of the question",
			"database_id":   "Database ID",
			"query":         "SQL query string",
			"description":   "Optional description",
			"collection_id": "Optional collection ID to save into",
			"display":       "Visualization type: table, bar, line, pie, scalar, etc. (default: table)",
		},
		Required: []string{"name", "database_id", "query"},
	},
	{
		Name:        mcp.ToolName("metabase_update_card"),
		Description: "Update a saved question/card (name, description, query, visualization)",
		Parameters: map[string]string{
			"card_id":     "Card ID",
			"name":        "New name",
			"description": "New description",
			"query":       "New SQL query",
			"database_id": "Database ID (required if changing query)",
			"display":     "Visualization type: table, bar, line, pie, scalar, etc.",
			"archived":    "Set to true to archive the card",
		},
		Required: []string{"card_id"},
	},
	{
		Name:        mcp.ToolName("metabase_delete_card"),
		Description: "Delete (archive) a saved question/card",
		Parameters:  map[string]string{"card_id": "Card ID"},
		Required:    []string{"card_id"},
	},

	// --- Dashboards ---
	{
		Name:        mcp.ToolName("metabase_list_dashboards"),
		Description: "List all Metabase analytics dashboards for reporting and data visualization",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("metabase_get_dashboard"),
		Description: "Get details of a dashboard including its cards and layout",
		Parameters:  map[string]string{"dashboard_id": "Dashboard ID"},
		Required:    []string{"dashboard_id"},
	},
	{
		Name:        mcp.ToolName("metabase_create_dashboard"),
		Description: "Create a new dashboard",
		Parameters: map[string]string{
			"name":          "Dashboard name",
			"description":   "Optional description",
			"collection_id": "Optional collection ID",
		},
		Required: []string{"name"},
	},
	{
		Name:        mcp.ToolName("metabase_update_dashboard"),
		Description: "Update a dashboard's name, description, or other properties",
		Parameters: map[string]string{
			"dashboard_id": "Dashboard ID",
			"name":         "New name",
			"description":  "New description",
			"archived":     "Set to true to archive the dashboard",
		},
		Required: []string{"dashboard_id"},
	},
	{
		Name:        mcp.ToolName("metabase_delete_dashboard"),
		Description: "Delete (archive) a dashboard",
		Parameters:  map[string]string{"dashboard_id": "Dashboard ID"},
		Required:    []string{"dashboard_id"},
	},
	{
		Name:        mcp.ToolName("metabase_add_card_to_dashboard"),
		Description: "Add a saved question/card to a dashboard",
		Parameters: map[string]string{
			"dashboard_id": "Dashboard ID",
			"card_id":      "Card ID to add",
			"size_x":       "Width in grid units (default: 6)",
			"size_y":       "Height in grid units (default: 4)",
			"row":          "Row position (default: 0)",
			"col":          "Column position (default: 0)",
		},
		Required: []string{"dashboard_id", "card_id"},
	},

	// --- Collections ---
	{
		Name:        mcp.ToolName("metabase_list_collections"),
		Description: "List all collections (folders for organizing questions and dashboards)",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("metabase_get_collection"),
		Description: "Get details and items in a specific collection",
		Parameters:  map[string]string{"collection_id": "Collection ID (use 'root' for the root collection)"},
		Required:    []string{"collection_id"},
	},
	{
		Name:        mcp.ToolName("metabase_create_collection"),
		Description: "Create a new collection for organizing questions and dashboards",
		Parameters: map[string]string{
			"name":        "Collection name",
			"description": "Optional description",
			"parent_id":   "Optional parent collection ID for nesting",
		},
		Required: []string{"name"},
	},
	{
		Name:        mcp.ToolName("metabase_update_collection"),
		Description: "Update a collection's name, description, or parent",
		Parameters: map[string]string{
			"collection_id": "Collection ID",
			"name":          "New name",
			"description":   "New description",
			"parent_id":     "New parent collection ID",
			"archived":      "Set to true to archive",
		},
		Required: []string{"collection_id"},
	},

	// --- Search ---
	{
		Name:        mcp.ToolName("metabase_search"),
		Description: "Search across all Metabase content (questions, dashboards, collections, tables, databases). Start here for BI and reporting workflows.",
		Parameters: map[string]string{
			"query":  "Search query string",
			"models": "Comma-separated types to search: card, dashboard, collection, table, database",
		},
		Required: []string{"query"},
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
