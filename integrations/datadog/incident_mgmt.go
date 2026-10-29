package datadog

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
	"github.com/google/uuid"
)

// ── Incident lookup ──────────────────────────────────────────────────

var publicIncidentIDPattern = regexp.MustCompile(`(?i)^(?:ir-|#|incident-)?(\d+)$`)

const (
	publicIDScanPageSize = 100
	publicIDScanMaxPages = 10
)

// resolveIncidentID accepts an incident UUID or a public id ("94", "IR-94",
// "#94", "incident-94") and returns the UUID the API expects.
func resolveIncidentID(ctx context.Context, d *dd, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("incident id is required")
	}
	if _, err := uuid.Parse(raw); err == nil {
		return raw, nil
	}
	m := publicIncidentIDPattern.FindStringSubmatch(raw)
	if m == nil {
		return raw, nil
	}
	want, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("incident id %q: %w", raw, err)
	}
	api := datadogV2.NewIncidentsApi(d.client)
	for page := 0; page < publicIDScanMaxPages; page++ {
		opts := datadogV2.NewListIncidentsOptionalParameters().
			WithPageSize(publicIDScanPageSize).
			WithPageOffset(int64(page * publicIDScanPageSize))
		resp, _, err := api.ListIncidents(ctx, *opts)
		if err != nil {
			return "", err
		}
		for _, inc := range resp.Data {
			if inc.Attributes != nil && inc.Attributes.PublicId != nil && *inc.Attributes.PublicId == want {
				return inc.Id, nil
			}
		}
		if len(resp.Data) < publicIDScanPageSize {
			break
		}
	}
	return "", fmt.Errorf("no incident with public id %d", want)
}

func incidentIDArg(ctx context.Context, d *dd, args map[string]any, key string) (string, error) {
	raw, err := mcp.ArgStr(args, key)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("parameter %q is required", key)
	}
	return resolveIncidentID(ctx, d, raw)
}

func getIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	incidentID, err := incidentIDArg(ctx, d, args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentsApi(d.client)
	resp, _, err := api.GetIncident(ctx, incidentID, *datadogV2.NewGetIncidentOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Incident update ──────────────────────────────────────────────────

func singleValueField(value string, kind datadogV2.IncidentFieldAttributesSingleValueType) datadogV2.IncidentFieldAttributes {
	f := datadogV2.IncidentFieldAttributesSingleValue{Value: *datadog.NewNullableString(&value)}
	if kind != "" {
		f.Type = &kind
	}
	return datadogV2.IncidentFieldAttributesSingleValueAsIncidentFieldAttributes(&f)
}

func multiValueField(values []string, kind datadogV2.IncidentFieldAttributesValueType) datadogV2.IncidentFieldAttributes {
	f := datadogV2.IncidentFieldAttributesMultipleValue{Value: *datadog.NewNullableList(&values)}
	if kind != "" {
		f.Type = &kind
	}
	return datadogV2.IncidentFieldAttributesMultipleValueAsIncidentFieldAttributes(&f)
}

func stringList(name string, raw []any) ([]string, error) {
	values := make([]string, 0, len(raw))
	for i, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("field %q: element %d is %T, not string", name, i, item)
		}
		values = append(values, s)
	}
	return values, nil
}

// parseIncidentFields maps {"name": "value"} to a single-value field and
// {"name": ["a", "b"]} to a multi-value field, leaving the field type to the
// org's field definition. {"name": {"type": "textbox", "value": "..."}} sets
// the type explicitly.
func parseIncidentFields(raw map[string]any) (map[string]datadogV2.IncidentFieldAttributes, error) {
	out := make(map[string]datadogV2.IncidentFieldAttributes, len(raw))
	for name, value := range raw {
		kind := ""
		if obj, ok := value.(map[string]any); ok {
			k, err := mcp.ArgStr(obj, "type")
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", name, err)
			}
			kind = k
			value = obj["value"]
		}
		switch v := value.(type) {
		case string:
			single := datadogV2.IncidentFieldAttributesSingleValueType(kind)
			if kind != "" && !single.IsValid() {
				return nil, fmt.Errorf("field %q: type %q must be dropdown or textbox for a single value", name, kind)
			}
			out[name] = singleValueField(v, single)
		case []any:
			values, err := stringList(name, v)
			if err != nil {
				return nil, err
			}
			multi := datadogV2.IncidentFieldAttributesValueType(kind)
			if kind != "" && !multi.IsValid() {
				return nil, fmt.Errorf("field %q: type %q must be multiselect, textarray, metrictag, or autocomplete for a list", name, kind)
			}
			out[name] = multiValueField(values, multi)
		default:
			return nil, fmt.Errorf("field %q: value must be a string or an array of strings, got %T", name, value)
		}
	}
	return out, nil
}

func parseTimeArg(args map[string]any, key string) (*time.Time, error) {
	raw, err := mcp.ArgStr(args, key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("parameter %q: must be RFC3339 (e.g. 2026-10-23T18:52:00Z): %w", key, err)
	}
	return &t, nil
}

func updateIncident(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	severity := r.Str("severity")
	status := r.Str("status")
	scope := r.Str("customer_impact_scope")
	commander := r.Str("commander_user_id")
	rawFields := r.Map("fields")
	_, impactedSet := args["customer_impacted"]
	impacted := r.Bool("customer_impacted")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	incidentID, err := incidentIDArg(ctx, d, args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}

	attrs := datadogV2.IncidentUpdateAttributes{}
	changed := false
	if title != "" {
		attrs.Title = datadog.PtrString(title)
		changed = true
	}
	if impactedSet {
		attrs.CustomerImpacted = datadog.PtrBool(impacted)
		changed = true
	}
	if scope != "" {
		attrs.CustomerImpactScope = datadog.PtrString(scope)
		changed = true
	}
	for key, set := range map[string]func(time.Time){
		"customer_impact_start": func(t time.Time) { attrs.CustomerImpactStart = *datadog.NewNullableTime(&t) },
		"customer_impact_end":   func(t time.Time) { attrs.CustomerImpactEnd = *datadog.NewNullableTime(&t) },
		"detected":              func(t time.Time) { attrs.Detected = *datadog.NewNullableTime(&t) },
	} {
		t, err := parseTimeArg(args, key)
		if err != nil {
			return mcp.ErrResult(err)
		}
		if t != nil {
			set(*t)
			changed = true
		}
	}

	fields, err := parseIncidentFields(rawFields)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if severity != "" {
		fields["severity"] = singleValueField(severity, datadogV2.INCIDENTFIELDATTRIBUTESSINGLEVALUETYPE_DROPDOWN)
	}
	if status != "" {
		fields["state"] = singleValueField(status, datadogV2.INCIDENTFIELDATTRIBUTESSINGLEVALUETYPE_DROPDOWN)
	}
	if len(fields) > 0 {
		attrs.Fields = fields
		changed = true
	}

	data := datadogV2.IncidentUpdateData{
		Id:         incidentID,
		Attributes: &attrs,
		Type:       datadogV2.INCIDENTTYPE_INCIDENTS,
	}
	if commander != "" {
		if _, err := uuid.Parse(commander); err != nil {
			return mcp.ErrResult(fmt.Errorf("parameter \"commander_user_id\": %q is not a Datadog user UUID (look it up with datadog_list_users)", commander))
		}
		rel := datadogV2.NewIncidentUpdateRelationships()
		userData := datadogV2.NewNullableRelationshipToUserData(commander, datadogV2.USERSTYPE_USERS)
		rel.SetCommanderUser(*datadogV2.NewNullableRelationshipToUser(*datadogV2.NewNullableNullableRelationshipToUserData(userData)))
		data.Relationships = rel
		changed = true
	}
	if !changed {
		return mcp.ErrResult(fmt.Errorf("nothing to update: set at least one field besides id"))
	}

	api := datadogV2.NewIncidentsApi(d.client)
	resp, _, err := api.UpdateIncident(ctx, incidentID, datadogV2.IncidentUpdateRequest{Data: data}, *datadogV2.NewUpdateIncidentOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Incident todos ───────────────────────────────────────────────────

func todoAttributes(args map[string]any, requireContent bool) (datadogV2.IncidentTodoAttributes, error) {
	r := mcp.NewArgs(args)
	content := r.Str("content")
	assignees := r.StrSlice("assignees")
	dueDate := r.Str("due_date")
	completed := r.Str("completed")
	if err := r.Err(); err != nil {
		return datadogV2.IncidentTodoAttributes{}, err
	}
	if requireContent && strings.TrimSpace(content) == "" {
		return datadogV2.IncidentTodoAttributes{}, fmt.Errorf("parameter \"content\" is required")
	}
	attrs := datadogV2.IncidentTodoAttributes{Content: content, Assignees: []datadogV2.IncidentTodoAssignee{}}
	for _, handle := range assignees {
		h := strings.TrimSpace(handle)
		if h == "" {
			continue
		}
		attrs.Assignees = append(attrs.Assignees, datadogV2.IncidentTodoAssigneeHandleAsIncidentTodoAssignee(&h))
	}
	if dueDate != "" {
		attrs.DueDate = *datadog.NewNullableString(&dueDate)
	}
	if completed != "" {
		attrs.Completed = *datadog.NewNullableString(&completed)
	}
	return attrs, nil
}

func getIncidentTodo(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	todoID := r.Str("todo_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).GetIncidentTodo(ctx, incidentID, todoID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createIncidentTodo(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	attrs, err := todoAttributes(args, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.IncidentTodoCreateRequest{Data: datadogV2.IncidentTodoCreateData{
		Attributes: attrs,
		Type:       datadogV2.INCIDENTTODOTYPE_INCIDENT_TODOS,
	}}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).CreateIncidentTodo(ctx, incidentID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateIncidentTodo(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	todoID := r.Str("todo_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	attrs, err := todoAttributes(args, true)
	if err != nil {
		return mcp.ErrResult(err)
	}
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.IncidentTodoPatchRequest{Data: datadogV2.IncidentTodoPatchData{
		Attributes: attrs,
		Type:       datadogV2.INCIDENTTODOTYPE_INCIDENT_TODOS,
	}}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).UpdateIncidentTodo(ctx, incidentID, todoID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteIncidentTodo(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	todoID := r.Str("todo_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := datadogV2.NewIncidentsApi(d.client).DeleteIncidentTodo(ctx, incidentID, todoID); err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted", "todo_id": todoID})
}

// ── Incident integrations, impacts, attachments, types ───────────────

func listIncidentIntegrations(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).ListIncidentIntegrations(ctx, incidentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getIncidentIntegration(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	integrationID := r.Str("integration_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).GetIncidentIntegration(ctx, incidentID, integrationID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func listIncidentImpacts(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).ListIncidentImpacts(ctx, incidentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createIncidentImpact(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if strings.TrimSpace(description) == "" {
		return mcp.ErrResult(fmt.Errorf("parameter \"description\" is required"))
	}
	start, err := parseTimeArg(args, "start_at")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if start == nil {
		return mcp.ErrResult(fmt.Errorf("parameter \"start_at\" is required (RFC3339)"))
	}
	end, err := parseTimeArg(args, "end_at")
	if err != nil {
		return mcp.ErrResult(err)
	}
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	attrs := datadogV2.IncidentImpactCreateAttributes{Description: description, StartAt: *start}
	if end != nil {
		attrs.EndAt = *datadog.NewNullableTime(end)
	}
	body := datadogV2.IncidentImpactCreateRequest{Data: datadogV2.IncidentImpactCreateData{
		Attributes: attrs,
		Type:       datadogV2.INCIDENTIMPACTTYPE_INCIDENT_IMPACTS,
	}}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).CreateIncidentImpact(ctx, incidentID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createIncidentAttachment(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	url := r.Str("url")
	title := r.Str("title")
	kind := r.Str("attachment_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if strings.TrimSpace(url) == "" {
		return mcp.ErrResult(fmt.Errorf("parameter \"url\" is required"))
	}
	if kind == "" {
		kind = string(datadogV2.ATTACHMENTDATAATTRIBUTESATTACHMENTTYPE_LINK)
	}
	attachmentType := datadogV2.AttachmentDataAttributesAttachmentType(kind)
	if !attachmentType.IsValid() {
		return mcp.ErrResult(fmt.Errorf("parameter \"attachment_type\": %q must be link or postmortem", kind))
	}
	incidentID, err := incidentIDArg(ctx, d, args, "incident_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	attachment := datadogV2.CreateAttachmentRequestDataAttributesAttachment{DocumentUrl: datadog.PtrString(url)}
	if title != "" {
		attachment.Title = datadog.PtrString(title)
	}
	body := datadogV2.CreateAttachmentRequest{Data: &datadogV2.CreateAttachmentRequestData{
		Attributes: &datadogV2.CreateAttachmentRequestDataAttributes{
			Attachment:     &attachment,
			AttachmentType: &attachmentType,
		},
		Type: datadogV2.INCIDENTATTACHMENTTYPE_INCIDENT_ATTACHMENTS,
	}}
	resp, _, err := datadogV2.NewIncidentsApi(d.client).CreateIncidentAttachment(ctx, incidentID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func listIncidentTypes(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	includeDeleted := r.Bool("include_deleted")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := datadogV2.NewListIncidentTypesOptionalParameters().WithIncludeDeleted(includeDeleted)
	resp, _, err := datadogV2.NewIncidentsApi(d.client).ListIncidentTypes(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
