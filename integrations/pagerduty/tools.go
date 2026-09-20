package pagerduty

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name:        mcp.ToolName("pagerduty_list_incidents"),
		Description: "List PagerDuty incidents, alerts, and on-call pages. Start here for incident response, outages, paging, and open triggered or acknowledged problems. Filter by status, urgency, or service.",
		Parameters: map[string]string{
			"statuses":    "Comma-separated statuses: triggered, acknowledged, resolved. Default triggered,acknowledged",
			"urgencies":   "Comma-separated urgencies: high, low",
			"service_ids": "Comma-separated PagerDuty service IDs",
			"since":       "RFC3339 start time filter",
			"until":       "RFC3339 end time filter",
			"limit":       "Page size (default 25, max 100)",
			"offset":      "Pagination offset (default 0)",
		},
	},
	{
		Name:        mcp.ToolName("pagerduty_get_incident"),
		Description: "Get a specific PagerDuty incident, outage, or page by ID, including status, urgency, assignments, and html_url. Use after list_incidents.",
		Parameters:  map[string]string{"incident_id": "PagerDuty incident ID"},
		Required:    []string{"incident_id"},
	},
	{
		Name:        mcp.ToolName("pagerduty_list_oncalls"),
		Description: "List current PagerDuty on-call rotations, schedules, and who is paging. Use for escalation, coverage, and who to contact during an incident.",
		Parameters: map[string]string{
			"user_ids":              "Comma-separated user IDs",
			"escalation_policy_ids": "Comma-separated escalation policy IDs",
			"schedule_ids":          "Comma-separated schedule IDs",
			"since":                 "RFC3339 start of on-call window",
			"until":                 "RFC3339 end of on-call window",
			"limit":                 "Page size (default 25, max 100)",
			"offset":                "Pagination offset (default 0)",
		},
	},
	{
		Name:        mcp.ToolName("pagerduty_list_services"),
		Description: "List PagerDuty services that route incidents and alerts. Use after list_incidents to map a service_id to a name.",
		Parameters: map[string]string{
			"query":  "Optional name search",
			"limit":  "Page size (default 25, max 100)",
			"offset": "Pagination offset (default 0)",
		},
	},
	{
		Name:        mcp.ToolName("pagerduty_add_incident_note"),
		Description: "Add a note or comment to a single PagerDuty incident. Requires from_email of a PagerDuty user. Use after list_incidents or get_incident.",
		Parameters: map[string]string{
			"incident_id": "PagerDuty incident ID",
			"content":     "Note text",
			"from_email":  "PagerDuty user email for the From header (overrides configured from_email)",
		},
		Required: []string{"incident_id", "content"},
	},
	{
		Name:        mcp.ToolName("pagerduty_acknowledge_incident"),
		Description: "Acknowledge a single PagerDuty incident so responders own the page. Explicit one-incident mutation; never bulk. Requires from_email. Use after list_incidents or get_incident.",
		Parameters: map[string]string{
			"incident_id": "PagerDuty incident ID",
			"from_email":  "PagerDuty user email for the From header (overrides configured from_email)",
		},
		Required: []string{"incident_id"},
	},
	{
		Name:        mcp.ToolName("pagerduty_resolve_incident"),
		Description: "Resolve a single PagerDuty incident after mitigation. Explicit one-incident mutation; never bulk. Requires from_email. Use after list_incidents or get_incident.",
		Parameters: map[string]string{
			"incident_id": "PagerDuty incident ID",
			"from_email":  "PagerDuty user email for the From header (overrides configured from_email)",
		},
		Required: []string{"incident_id"},
	},
}
