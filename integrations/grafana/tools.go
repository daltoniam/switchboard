package grafana

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        "grafana_search_dashboards",
		Description: "Search Grafana dashboards by title, tag or folder. Start here for Grafana: find a dashboard uid to read with grafana_get_dashboard or change with grafana_save_dashboard. Returns uid, title, url, folder and tags for each dashboard.",
		Parameters: map[string]string{
			"query":       "Optional text matched against dashboard titles",
			"tags":        "Optional array of tags; dashboards must have all of them",
			"folder_uids": "Optional array of folder uids from grafana_list_folders",
			"starred":     "Only dashboards the service account starred (default false)",
			"limit":       "Dashboards per page, 1-5000 (default 50)",
			"page":        "Page number, starting at 1",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "grafana_get_dashboard",
		Description: "Get a Grafana dashboard's full JSON model (panels, queries, variables, time range) with its folder and version. Use after grafana_search_dashboards. To change a dashboard, edit the returned dashboard and pass it to grafana_save_dashboard.",
		Parameters: map[string]string{
			"uid": "Dashboard uid from grafana_search_dashboards",
		},
		Required: []string{"uid"},
		Access:   mcp.AccessRead,
	},
	{
		Name: "grafana_save_dashboard",
		Description: "Create or update (save, import) a Grafana dashboard from its JSON model, optionally in a folder. WRITE: changes a shared dashboard everyone sees; Grafana keeps earlier versions in the dashboard's version history. " +
			"Without overwrite, an update succeeds only when the dashboard's version still matches the stored one, so a dashboard someone else changed is not replaced. " +
			"An exported dashboard with __inputs (such as ${DS_PROMETHEUS}) needs inputs mapping each name to a data source uid. Use after grafana_get_dashboard, grafana_list_folders or grafana_list_datasources.",
		Parameters: map[string]string{
			"dashboard":  "Dashboard JSON model as an object or JSON string. The {dashboard, meta} result of grafana_get_dashboard is accepted too. Omit uid to create a new dashboard",
			"folder_uid": "Optional folder uid from grafana_list_folders (default: the dashboard's current folder, or General for a new dashboard)",
			"overwrite":  "Replace the stored dashboard even if its version or title clash (default false)",
			"message":    "Optional version history note describing the change",
			"inputs":     `Optional object mapping each __inputs name to its value, e.g. {"DS_PROMETHEUS":"<data source uid>"}`,
		},
		Required: []string{"dashboard"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "grafana_list_folders",
		Description: "List Grafana dashboard folders with uid and title. Use to choose folder_uid for grafana_save_dashboard or folder_uids for grafana_search_dashboards.",
		Parameters: map[string]string{
			"parent_uid": "Optional parent folder uid, to list nested folders",
			"limit":      "Folders per page, 1-1000 (default 100)",
			"page":       "Page number, starting at 1",
		},
		Access: mcp.AccessRead,
	},
	{
		Name:        "grafana_list_datasources",
		Description: "List Grafana data sources (Prometheus, Loki, Tempo, SQL and others) with uid, name, type and which is the default. Use to find datasource_uid for grafana_query_datasource or the values for dashboard inputs.",
		Access:      mcp.AccessRead,
	},
	{
		Name: "grafana_query_datasource",
		Description: "Run a metrics or logs query (PromQL for Prometheus, LogQL for Loki) through a Grafana data source and return data frames of timestamps and values. " +
			"Set instant for a single current value per series; otherwise returns a range over from..to. Use after grafana_list_datasources. " +
			"WRITE: Grafana runs the query with the data source's own database privileges and does not make it read-only, so raw SQL, Flux or InfluxQL may change data.",
		Parameters: map[string]string{
			"datasource_uid":  "Data source uid from grafana_list_datasources",
			"expr":            "Query expression, e.g. PromQL rate(http_requests_total[5m]) or LogQL {app=\"api\"}",
			"query":           `Optional object of extra query fields for other data source types, e.g. {"rawSql":"select 1","format":"table"}; merged into the query (use max_data_points and interval_ms for those limits)`,
			"from":            "Range start: now-1h, an epoch in milliseconds, or RFC3339 (default now-1h)",
			"to":              "Range end: now, an epoch in milliseconds, or RFC3339 (default now)",
			"instant":         "Instant query: one value per series at to (default false: range query)",
			"max_data_points": "Maximum points per series, 1-2000 (default 300)",
			"interval_ms":     "Optional step between points in milliseconds (default: the range divided by max_data_points, at least 1000)",
		},
		Required: []string{"datasource_uid"},
		Access:   mcp.AccessWrite,
	},
	{
		Name:        "grafana_get_health",
		Description: "Check that Grafana is reachable and the service account token works: Grafana version, database status and the token's organization. Use when other Grafana tools fail, for example when a Grafana Cloud free stack is asleep.",
		Access:      mcp.AccessRead,
	},
}

var dispatch = map[mcp.ToolName]handlerFunc{
	"grafana_search_dashboards": searchDashboards,
	"grafana_get_dashboard":     getDashboard,
	"grafana_save_dashboard":    saveDashboard,
	"grafana_list_folders":      listFolders,
	"grafana_list_datasources":  listDatasources,
	"grafana_query_datasource":  queryDatasource,
	"grafana_get_health":        getHealth,
}
