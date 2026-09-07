package servicenow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

const (
	incidentListFields  = "sys_id,number,short_description,state,priority,urgency,impact,assigned_to,assignment_group,caller_id,category,sys_updated_on,sys_created_on"
	incidentGetFields   = incidentListFields + ",description,close_notes,close_code,subcategory,cmdb_ci,opened_at,resolved_at,closed_at"
	problemListFields   = "sys_id,number,short_description,state,priority,assigned_to,assignment_group,sys_updated_on,sys_created_on"
	problemGetFields    = problemListFields + ",description,workaround,cause_notes,fix_notes"
	changeListFields    = "sys_id,number,short_description,state,priority,type,assigned_to,assignment_group,start_date,end_date,sys_updated_on"
	changeGetFields     = changeListFields + ",description,justification,implementation_plan,backout_plan,test_plan,risk,impact"
	requestListFields   = "sys_id,number,short_description,request_state,stage,priority,requested_for,assignment_group,sys_updated_on"
	requestGetFields    = requestListFields + ",description,special_instructions"
	knowledgeListFields = "sys_id,number,short_description,workflow_state,kb_knowledge_base,category,sys_view_count,sys_updated_on,published"
	knowledgeGetFields  = knowledgeListFields + ",text,wiki,meta,author,valid_to"
	userListFields      = "sys_id,user_name,name,email,title,department,active,sys_updated_on"
	userGetFields       = userListFields + ",phone,mobile_phone,manager,location,company"
	groupListFields     = "sys_id,name,description,email,manager,active,sys_updated_on"
	ciListFields        = "sys_id,name,sys_class_name,operational_status,install_status,assigned_to,support_group,sys_updated_on"
	ciGetFields         = ciListFields + ",short_description,ip_address,fqdn,serial_number,manufacturer,model_id,location,owned_by"
	commentListFields   = "sys_id,element,element_id,value,sys_created_on,sys_created_by"
	attachmentFields    = "sys_id,file_name,content_type,size_bytes,table_name,table_sys_id,sys_created_on,sys_created_by"
)

func listTable(ctx context.Context, s *servicenow, table string, args map[string]any, defaultFields string) (*mcp.ToolResult, error) {
	if err := validTable(table); err != nil {
		return mcp.ErrResult(err)
	}
	params, err := listQueryParams(args, defaultFields)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/now/table/%s%s", url.PathEscape(table), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTable(ctx context.Context, s *servicenow, table, sysID string, args map[string]any, defaultFields string) (*mcp.ToolResult, error) {
	if err := validTable(table); err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	fields := r.Str("fields")
	displayValue := r.Str("display_value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if fields == "" {
		fields = defaultFields
	}
	if displayValue == "" {
		displayValue = "all"
	}
	params := map[string]string{
		"sysparm_fields":                 fields,
		"sysparm_display_value":          displayValue,
		"sysparm_exclude_reference_link": "true",
	}
	data, err := s.get(ctx, "/api/now/table/%s/%s%s", url.PathEscape(table), url.PathEscape(sysID), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func parseDataJSON(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return nil, fmt.Errorf("invalid JSON for data: %w", err)
	}
	return body, nil
}

func mergeField(body map[string]any, key, value string) {
	if value != "" {
		body[key] = value
	}
}

func listIncidents(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	return listTable(ctx, s, "incident", args, incidentListFields)
}

func getIncident(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTable(ctx, s, "incident", sysID, args, incidentGetFields)
}

func createIncident(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	shortDescription := r.Str("short_description")
	description := r.Str("description")
	urgency := r.Str("urgency")
	impact := r.Str("impact")
	priority := r.Str("priority")
	callerID := r.Str("caller_id")
	assignmentGroup := r.Str("assignment_group")
	assignedTo := r.Str("assigned_to")
	category := r.Str("category")
	subcategory := r.Str("subcategory")
	cmdbCI := r.Str("cmdb_ci")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body, err := parseDataJSON(dataStr)
	if err != nil {
		return mcp.ErrResult(err)
	}
	mergeField(body, "short_description", shortDescription)
	mergeField(body, "description", description)
	mergeField(body, "urgency", urgency)
	mergeField(body, "impact", impact)
	mergeField(body, "priority", priority)
	mergeField(body, "caller_id", callerID)
	mergeField(body, "assignment_group", assignmentGroup)
	mergeField(body, "assigned_to", assignedTo)
	mergeField(body, "category", category)
	mergeField(body, "subcategory", subcategory)
	mergeField(body, "cmdb_ci", cmdbCI)
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("provide at least one field to create"))
	}
	data, err := s.post(ctx, "/api/now/table/incident", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateIncident(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	shortDescription := r.Str("short_description")
	description := r.Str("description")
	state := r.Str("state")
	urgency := r.Str("urgency")
	impact := r.Str("impact")
	priority := r.Str("priority")
	assignedTo := r.Str("assigned_to")
	assignmentGroup := r.Str("assignment_group")
	closeCode := r.Str("close_code")
	closeNotes := r.Str("close_notes")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body, err := parseDataJSON(dataStr)
	if err != nil {
		return mcp.ErrResult(err)
	}
	mergeField(body, "short_description", shortDescription)
	mergeField(body, "description", description)
	mergeField(body, "state", state)
	mergeField(body, "urgency", urgency)
	mergeField(body, "impact", impact)
	mergeField(body, "priority", priority)
	mergeField(body, "assigned_to", assignedTo)
	mergeField(body, "assignment_group", assignmentGroup)
	mergeField(body, "close_code", closeCode)
	mergeField(body, "close_notes", closeNotes)
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("provide at least one field to update"))
	}
	data, err := s.patch(ctx, "/api/now/table/incident/"+url.PathEscape(sysID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProblems(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	return listTable(ctx, s, "problem", args, problemListFields)
}

func getProblem(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTable(ctx, s, "problem", sysID, args, problemGetFields)
}

func listChangeRequests(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	return listTable(ctx, s, "change_request", args, changeListFields)
}

func getChangeRequest(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTable(ctx, s, "change_request", sysID, args, changeGetFields)
}

func listCatalogRequests(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	return listTable(ctx, s, "sc_request", args, requestListFields)
}

func getCatalogRequest(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTable(ctx, s, "sc_request", sysID, args, requestGetFields)
}

func listKnowledgeArticles(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	return listTable(ctx, s, "kb_knowledge", args, knowledgeListFields)
}

func getKnowledgeArticle(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTable(ctx, s, "kb_knowledge", sysID, args, knowledgeGetFields)
}

func listUsers(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	return listTable(ctx, s, "sys_user", args, userListFields)
}

func getUser(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTable(ctx, s, "sys_user", sysID, args, userGetFields)
}

func listGroups(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	return listTable(ctx, s, "sys_user_group", args, groupListFields)
}

func listCIs(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		table = "cmdb_ci"
	}
	return listTable(ctx, s, table, args, ciListFields)
}

func getCI(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	table := r.Str("table")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		table = "cmdb_ci"
	}
	return getTable(ctx, s, table, sysID, args, ciGetFields)
}

func listRecords(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return listTable(ctx, s, table, args, "")
}

func getRecord(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return getTable(ctx, s, table, sysID, args, "")
}

func createRecord(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validTable(table); err != nil {
		return mcp.ErrResult(err)
	}
	body, err := parseDataJSON(dataStr)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("provide at least one field to create"))
	}
	data, err := s.post(ctx, "/api/now/table/"+url.PathEscape(table), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateRecord(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	sysID := r.Str("sys_id")
	dataStr := r.Str("data")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validTable(table); err != nil {
		return mcp.ErrResult(err)
	}
	body, err := parseDataJSON(dataStr)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("provide at least one field to update"))
	}
	data, err := s.patch(ctx, fmt.Sprintf("/api/now/table/%s/%s", url.PathEscape(table), url.PathEscape(sysID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func aggregate(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	query := r.Str("query")
	avgFields := r.Str("avg_fields")
	sumFields := r.Str("sum_fields")
	minFields := r.Str("min_fields")
	maxFields := r.Str("max_fields")
	groupBy := r.Str("group_by")
	count := r.Bool("count")
	if _, ok := args["count"]; !ok {
		count = true
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validTable(table); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"sysparm_query": query,
		"sysparm_count": fmt.Sprintf("%t", count),
	}
	if avgFields != "" {
		params["sysparm_avg_fields"] = avgFields
	}
	if sumFields != "" {
		params["sysparm_sum_fields"] = sumFields
	}
	if minFields != "" {
		params["sysparm_min_fields"] = minFields
	}
	if maxFields != "" {
		params["sysparm_max_fields"] = maxFields
	}
	if groupBy != "" {
		params["sysparm_group_by"] = groupBy
	}
	data, err := s.get(ctx, "/api/now/stats/%s%s", url.PathEscape(table), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listComments(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	extra := r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	query := "element_id=" + sysID + "^ORDERBYDESCsys_created_on"
	if extra != "" {
		query += "^" + extra
	}
	cloned := cloneArgs(args)
	cloned["query"] = query
	cloned["fields"] = commentListFields
	return listTable(ctx, s, "sys_journal_field", cloned, commentListFields)
}

func addComment(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sysID := r.Str("sys_id")
	table := r.Str("table")
	comment := r.Str("comment")
	workNote := r.Str("work_note")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if table == "" {
		table = "incident"
	}
	if err := validTable(table); err != nil {
		return mcp.ErrResult(err)
	}
	if comment == "" && workNote == "" {
		return mcp.ErrResult(fmt.Errorf("provide comment and/or work_note"))
	}
	body := map[string]any{}
	if comment != "" {
		body["comments"] = comment
	}
	if workNote != "" {
		body["work_notes"] = workNote
	}
	data, err := s.patch(ctx, fmt.Sprintf("/api/now/table/%s/%s", url.PathEscape(table), url.PathEscape(sysID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAttachments(ctx context.Context, s *servicenow, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	table := r.Str("table")
	sysID := r.Str("sys_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := validTable(table); err != nil {
		return mcp.ErrResult(err)
	}
	cloned := cloneArgs(args)
	cloned["query"] = "table_name=" + table + "^table_sys_id=" + sysID
	cloned["fields"] = attachmentFields
	return listTable(ctx, s, "sys_attachment", cloned, attachmentFields)
}

func cloneArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args)+2)
	for k, v := range args {
		out[k] = v
	}
	return out
}
