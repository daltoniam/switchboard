package gong

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("gong_list_calls"), Description: "List Gong sales call recordings and conversations in a date range. Start here for call review, coaching, deal conversations, and conversation intelligence workflows.",
		Parameters: map[string]string{
			"from_date_time": "Start of range (ISO 8601, required by Gong; max 90-day window)",
			"to_date_time":   "End of range (ISO 8601)",
			"workspace_id":   "Optional workspace ID filter",
			"cursor":         "Pagination cursor from a previous response",
		},
		Required: []string{"from_date_time", "to_date_time"},
	},
	{
		Name: mcp.ToolName("gong_get_call"), Description: "Get metadata for a specific Gong call recording by ID. Use after list_calls.",
		Parameters: map[string]string{"call_id": "Gong call ID"},
		Required:   []string{"call_id"},
	},
	{
		Name: mcp.ToolName("gong_list_calls_extensive"), Description: "Retrieve extensive Gong call details (parties, content, media, collaboration) for filtered calls. Prefer over repeated get_call when many fields are needed.",
		Parameters: map[string]string{
			"from_date_time":   "Start of range (ISO 8601)",
			"to_date_time":     "End of range (ISO 8601)",
			"call_ids":         "Optional JSON array of call IDs (e.g. [\"123\",\"456\"])",
			"workspace_id":     "Optional workspace ID",
			"cursor":           "Pagination cursor",
			"content_selector": "Optional JSON contentSelector object controlling returned sections",
		},
	},
	{
		Name: mcp.ToolName("gong_get_transcripts"), Description: "Fetch call transcripts and spoken sentences for Gong recordings. Use after list_calls or get_call when you need conversation text.",
		Parameters: map[string]string{
			"from_date_time": "Start of range (ISO 8601)",
			"to_date_time":   "End of range (ISO 8601)",
			"call_ids":       "Optional JSON array of call IDs",
			"workspace_id":   "Optional workspace ID",
			"cursor":         "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("gong_list_users"), Description: "List Gong users, sales reps, and teammates in the company workspace",
		Parameters: map[string]string{
			"cursor": "Pagination cursor",
		},
	},
	{
		Name: mcp.ToolName("gong_get_user"), Description: "Get a Gong user by ID. Use after list_users.",
		Parameters: map[string]string{"user_id": "Gong user ID"},
		Required:   []string{"user_id"},
	},
	{
		Name: mcp.ToolName("gong_list_users_extensive"), Description: "Batch-fetch extensive Gong user profiles by ID list",
		Parameters: map[string]string{
			"user_ids": "JSON array of user IDs (e.g. [\"1\",\"2\"])",
			"cursor":   "Pagination cursor",
		},
		Required: []string{"user_ids"},
	},
	{
		Name: mcp.ToolName("gong_list_workspaces"), Description: "List Gong workspaces available to the API key",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("gong_list_library_folders"), Description: "List Gong library folders used to organize shared call clips and coaching content",
		Parameters: map[string]string{
			"workspace_id": "Optional workspace ID",
		},
	},
	{
		Name: mcp.ToolName("gong_get_library_folder"), Description: "Get contents of a Gong library folder by ID. Use after list_library_folders.",
		Parameters: map[string]string{
			"folder_id":    "Library folder ID",
			"workspace_id": "Optional workspace ID",
		},
		Required: []string{"folder_id"},
	},
	{
		Name: mcp.ToolName("gong_list_stats_activity"), Description: "Get Gong activity statistics for users over a date range (calls, emails, meetings)",
		Parameters: map[string]string{
			"from_date":    "Start date (YYYY-MM-DD)",
			"to_date":      "End date (YYYY-MM-DD)",
			"workspace_id": "Optional workspace ID",
			"user_ids":     "Optional JSON array of user IDs",
		},
		Required: []string{"from_date", "to_date"},
	},
	{
		Name: mcp.ToolName("gong_list_stats_interaction"), Description: "Get Gong interaction statistics (talk ratio, patience, questions) for coaching analysis",
		Parameters: map[string]string{
			"from_date":    "Start date (YYYY-MM-DD)",
			"to_date":      "End date (YYYY-MM-DD)",
			"workspace_id": "Optional workspace ID",
			"user_ids":     "Optional JSON array of user IDs",
		},
		Required: []string{"from_date", "to_date"},
	},
	{
		Name: mcp.ToolName("gong_list_stats_scorecards"), Description: "Get Gong scorecard review statistics for call coaching quality scores",
		Parameters: map[string]string{
			"from_date":    "Start date (YYYY-MM-DD)",
			"to_date":      "End date (YYYY-MM-DD)",
			"workspace_id": "Optional workspace ID",
			"user_ids":     "Optional JSON array of user IDs",
		},
		Required: []string{"from_date", "to_date"},
	},
	{
		Name: mcp.ToolName("gong_list_logs"), Description: "List Gong API and activity logs for auditing integration usage",
		Parameters: map[string]string{
			"from_date_time": "Start of range (ISO 8601)",
			"to_date_time":   "End of range (ISO 8601)",
			"cursor":         "Pagination cursor",
		},
		Required: []string{"from_date_time", "to_date_time"},
	},
	{
		Name: mcp.ToolName("gong_list_data_privacy"), Description: "List Gong data-privacy deletion requests and compliance status",
		Parameters: map[string]string{
			"cursor": "Pagination cursor",
		},
	},
}
