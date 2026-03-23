package mixpanel

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Insights ────────────────────────────────────────────────────
	{
		Name:        "mixpanel_query_insights",
		Description: "Run a saved Mixpanel Insights report by bookmark ID. Returns the computed results of the saved report.",
		Parameters: map[string]string{
			"bookmark_id": "The saved report bookmark ID",
			"project_id":  "Project ID (defaults to configured project)",
		},
		Required: []string{"bookmark_id"},
	},

	// ── Funnels ─────────────────────────────────────────────────────
	{
		Name:        "mixpanel_query_funnels",
		Description: "Query funnel conversion data. Returns step-by-step conversion rates and counts for the specified funnel over a date range.",
		Parameters: map[string]string{
			"funnel_id":  "Funnel ID to query",
			"from_date":  "Start date (YYYY-MM-DD)",
			"to_date":    "End date (YYYY-MM-DD)",
			"length":     "Funnel conversion window in days",
			"interval":   "Grouping interval: day, week, or month",
			"unit":       "Alternate time unit for the conversion window",
			"on":         "JSON string — property expression to segment the funnel by",
			"where":      "Filter expression (e.g. properties[\"country\"] == \"US\")",
			"limit":      "Max number of segmentation values to return",
			"project_id": "Project ID (defaults to configured project)",
		},
		Required: []string{"funnel_id", "from_date", "to_date"},
	},

	// ── Retention ───────────────────────────────────────────────────
	{
		Name:        "mixpanel_query_retention",
		Description: "Query user retention data. Returns cohort-based retention showing how many users come back over time.",
		Parameters: map[string]string{
			"from_date":      "Start date (YYYY-MM-DD)",
			"to_date":        "End date (YYYY-MM-DD)",
			"retention_type": "Type: birth (first event) or compounded (any event)",
			"born_event":     "Event that defines the cohort entry (birth event)",
			"event":          "Event that counts as a return visit",
			"born_where":     "Filter expression for the birth event",
			"where":          "Filter expression for the return event",
			"interval":       "Time interval: 1 (day) or 7 (week)",
			"interval_count": "Number of intervals to measure retention over",
			"unit":           "Time unit: day, week, or month",
			"on":             "JSON string — property expression to segment by",
			"limit":          "Max number of segmentation values",
			"project_id":     "Project ID (defaults to configured project)",
		},
		Required: []string{"from_date", "to_date"},
	},

	// ── Segmentation ────────────────────────────────────────────────
	{
		Name:        "mixpanel_query_segmentation",
		Description: "Query event segmentation data. Returns event counts over time, optionally broken down by a property.",
		Parameters: map[string]string{
			"event":      "Event name to segment",
			"from_date":  "Start date (YYYY-MM-DD)",
			"to_date":    "End date (YYYY-MM-DD)",
			"on":         "JSON string — property expression to break down by (e.g. 'properties[\"country\"]')",
			"where":      "Filter expression (e.g. properties[\"plan\"] == \"premium\")",
			"unit":       "Time unit: minute, hour, day, week, or month",
			"type":       "Analysis type: general, unique, or average",
			"limit":      "Max number of segmentation values",
			"project_id": "Project ID (defaults to configured project)",
		},
		Required: []string{"event", "from_date", "to_date"},
	},

	// ── Event Properties ────────────────────────────────────────────
	{
		Name:        "mixpanel_query_event_properties",
		Description: "Query event property values over time. Returns a breakdown of a specific property for a given event.",
		Parameters: map[string]string{
			"event":      "Event name",
			"name":       "Property name to query (e.g. 'country' or 'browser')",
			"from_date":  "Start date (YYYY-MM-DD)",
			"to_date":    "End date (YYYY-MM-DD)",
			"values":     "JSON array of specific property values to return",
			"type":       "Analysis type: general, unique, or average",
			"unit":       "Time unit: minute, hour, day, week, or month",
			"limit":      "Max number of property values",
			"project_id": "Project ID (defaults to configured project)",
		},
		Required: []string{"event", "name", "from_date", "to_date"},
	},

	// ── Profiles (Engage) ───────────────────────────────────────────
	{
		Name:        "mixpanel_query_profiles",
		Description: "Query user profiles from the Engage API. Returns user profile data with properties. Supports filtering, pagination, and property selection.",
		Parameters: map[string]string{
			"distinct_id":       "Single distinct ID to look up",
			"distinct_ids":      "JSON array of distinct IDs to look up",
			"where":             "Filter expression (e.g. properties[\"plan\"] == \"premium\")",
			"output_properties": "JSON array of property names to include in results",
			"session_id":        "Session ID from a previous response for pagination",
			"page":              "Page number (use with session_id for pagination)",
			"filter_by_cohort":  "JSON object with cohort filter (e.g. {\"id\": 1234})",
			"include_all_users": "Include users without profile properties (true/false)",
			"project_id":        "Project ID (defaults to configured project)",
		},
	},
}
