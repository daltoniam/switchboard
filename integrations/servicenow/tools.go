package servicenow

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	{
		Name: mcp.ToolName("servicenow_list_incidents"), Description: "List ServiceNow ITSM incidents, tickets, outages, and support issues. Start here for incident triage, ticket lookup, and IT service management workflows.",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. active=true^priority=1, state=1, number=INC0010001). Use ORDERBYsys_updated_onDESC for newest first.",
			"fields":        "Comma-separated fields to return (default: sys_id,number,short_description,state,priority,urgency,impact,assigned_to,assignment_group,caller_id,category,sys_updated_on,sys_created_on)",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_get_incident"), Description: "Get a ServiceNow incident ticket by sys_id. Use after list_incidents.",
		Parameters: map[string]string{
			"sys_id":        "Incident sys_id",
			"fields":        "Comma-separated fields to return (default: common incident fields plus description, close_notes, close_code)",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_create_incident"), Description: "Create a ServiceNow incident ticket. Use after list_incidents when opening a new issue, outage, or support request.",
		Parameters: map[string]string{
			"short_description": "Incident summary (required)",
			"description":       "Detailed description",
			"urgency":           "Urgency (1 high, 2 medium, 3 low)",
			"impact":            "Impact (1 high, 2 medium, 3 low)",
			"priority":          "Priority (1-5). ServiceNow may compute this from urgency and impact.",
			"caller_id":         "Caller sys_id or user_name",
			"assignment_group":  "Assignment group sys_id or name",
			"assigned_to":       "Assignee sys_id or user_name",
			"category":          "Category (e.g. network, hardware, software)",
			"subcategory":       "Subcategory",
			"cmdb_ci":           "Related configuration item sys_id",
			"data":              "JSON object of extra field values merged into the create body",
		},
		Required: []string{"short_description"},
	},
	{
		Name: mcp.ToolName("servicenow_update_incident"), Description: "Update a ServiceNow incident ticket (state, assignment, work notes, resolution). Use after get_incident.",
		Parameters: map[string]string{
			"sys_id":            "Incident sys_id",
			"short_description": "New summary",
			"description":       "New description",
			"state":             "State value (1 New, 2 In Progress, 3 On Hold, 6 Resolved, 7 Closed, 8 Canceled)",
			"urgency":           "Urgency (1-3)",
			"impact":            "Impact (1-3)",
			"priority":          "Priority (1-5)",
			"assigned_to":       "Assignee sys_id or user_name",
			"assignment_group":  "Assignment group sys_id or name",
			"close_code":        "Resolution code when resolving",
			"close_notes":       "Resolution notes when resolving",
			"data":              "JSON object of extra field values merged into the update body",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_problems"), Description: "List ServiceNow problem records for root-cause analysis and known errors",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. active=true)",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_get_problem"), Description: "Get a ServiceNow problem record by sys_id. Use after list_problems.",
		Parameters: map[string]string{
			"sys_id":        "Problem sys_id",
			"fields":        "Comma-separated fields to return",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_change_requests"), Description: "List ServiceNow change requests, CAB approvals, and planned IT changes",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. state=-2^type=normal)",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_get_change_request"), Description: "Get a ServiceNow change request by sys_id. Use after list_change_requests.",
		Parameters: map[string]string{
			"sys_id":        "Change request sys_id",
			"fields":        "Comma-separated fields to return",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_catalog_requests"), Description: "List ServiceNow service catalog requests (sc_request) and fulfillment tickets",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. request_state=in_process)",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_get_catalog_request"), Description: "Get a ServiceNow catalog request by sys_id. Use after list_catalog_requests.",
		Parameters: map[string]string{
			"sys_id":        "Catalog request sys_id",
			"fields":        "Comma-separated fields to return",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_knowledge_articles"), Description: "List ServiceNow knowledge base articles, runbooks, and how-to documentation",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. workflow_state=published^short_descriptionLIKEVPN)",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_get_knowledge_article"), Description: "Get a ServiceNow knowledge article by sys_id including body text. Use after list_knowledge_articles.",
		Parameters: map[string]string{
			"sys_id":        "Knowledge article sys_id",
			"fields":        "Comma-separated fields to return",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_users"), Description: "List ServiceNow users, assignees, callers, and ITIL operators",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. active=true^user_name=alice, email=alice@example.com)",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_get_user"), Description: "Get a ServiceNow user by sys_id. Use after list_users.",
		Parameters: map[string]string{
			"sys_id":        "User sys_id",
			"fields":        "Comma-separated fields to return",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_groups"), Description: "List ServiceNow assignment groups and support teams",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. active=true^nameLIKEnetwork)",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_list_cis"), Description: "List ServiceNow CMDB configuration items, assets, servers, and services",
		Parameters: map[string]string{
			"query":         "Encoded query (e.g. nameLIKEprod-web, operational_status=1)",
			"table":         "CMDB table (default cmdb_ci). Use cmdb_ci_server, cmdb_ci_linux_server, cmdb_ci_service, etc.",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
	},
	{
		Name: mcp.ToolName("servicenow_get_ci"), Description: "Get a ServiceNow CMDB configuration item by sys_id. Use after list_cis.",
		Parameters: map[string]string{
			"sys_id":        "CI sys_id",
			"table":         "CMDB table (default cmdb_ci)",
			"fields":        "Comma-separated fields to return",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_records"), Description: "List records from any ServiceNow table via the Table API. Prefer typed tools (list_incidents, list_problems) when available.",
		Parameters: map[string]string{
			"table":         "Table name (e.g. incident, problem, change_request, sc_req_item, cmdb_ci)",
			"query":         "Encoded query (sysparm_query)",
			"fields":        "Comma-separated fields to return",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"table"},
	},
	{
		Name: mcp.ToolName("servicenow_get_record"), Description: "Get a single ServiceNow table record by table and sys_id. Use after list_records.",
		Parameters: map[string]string{
			"table":         "Table name",
			"sys_id":        "Record sys_id",
			"fields":        "Comma-separated fields to return",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"table", "sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_create_record"), Description: "Create a record in any ServiceNow table. Provide field values as JSON.",
		Parameters: map[string]string{
			"table": "Table name (e.g. incident, problem)",
			"data":  "JSON object of field values (e.g. {\"short_description\":\"VPN down\"})",
		},
		Required: []string{"table", "data"},
	},
	{
		Name: mcp.ToolName("servicenow_update_record"), Description: "Update (PATCH) a ServiceNow table record. Send only fields to change.",
		Parameters: map[string]string{
			"table":  "Table name",
			"sys_id": "Record sys_id",
			"data":   "JSON object of fields to update",
		},
		Required: []string{"table", "sys_id", "data"},
	},
	{
		Name: mcp.ToolName("servicenow_aggregate"), Description: "Aggregate ServiceNow table stats (COUNT, AVG, SUM, MIN, MAX) for incident volume, backlog, and reporting",
		Parameters: map[string]string{
			"table":      "Table name (e.g. incident)",
			"query":      "Encoded query filter",
			"count":      "If true, return COUNT (default true)",
			"avg_fields": "Comma-separated fields to average",
			"sum_fields": "Comma-separated fields to sum",
			"min_fields": "Comma-separated fields for MIN",
			"max_fields": "Comma-separated fields for MAX",
			"group_by":   "Comma-separated group-by fields (e.g. assignment_group,priority)",
		},
		Required: []string{"table"},
	},
	{
		Name: mcp.ToolName("servicenow_list_comments"), Description: "List work notes and additional comments on a ServiceNow task (incident, problem, change). Use after get_incident.",
		Parameters: map[string]string{
			"sys_id":        "Parent task sys_id (incident, problem, change_request, etc.)",
			"query":         "Extra encoded query (AND-ed with element_id)",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_add_comment"), Description: "Add a work note or additional comment to a ServiceNow incident or task. Use after get_incident.",
		Parameters: map[string]string{
			"sys_id":    "Incident or task sys_id",
			"table":     "Table to update (default incident)",
			"comment":   "Customer-visible additional comments",
			"work_note": "Internal work notes (not customer-visible)",
		},
		Required: []string{"sys_id"},
	},
	{
		Name: mcp.ToolName("servicenow_list_attachments"), Description: "List file attachments on a ServiceNow record. Use after get_incident or get_record.",
		Parameters: map[string]string{
			"table":         "Table name (e.g. incident)",
			"sys_id":        "Parent record sys_id",
			"limit":         "Max records per page (default 25)",
			"offset":        "Pagination offset",
			"display_value": "Return display values: true, false, or all (default all)",
		},
		Required: []string{"table", "sys_id"},
	},
}
