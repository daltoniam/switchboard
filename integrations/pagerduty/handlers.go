package pagerduty

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func listIncidents(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	statuses := r.Str("statuses")
	urgencies := r.Str("urgencies")
	serviceIDs := r.Str("service_ids")
	since := r.Str("since")
	until := r.Str("until")
	limit := r.OptInt("limit", defaultLimit)
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if statuses == "" {
		statuses = "triggered,acknowledged"
	}
	vals := paginationQuery(limit, offset)
	addCSV(vals, "statuses[]", statuses)
	addCSV(vals, "urgencies[]", urgencies)
	addCSV(vals, "service_ids[]", serviceIDs)
	if since != "" {
		vals.Set("since", since)
	}
	if until != "" {
		vals.Set("until", until)
	}
	data, err := p.get(ctx, "/incidents?%s", vals.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIncident(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("incident_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("incident_id is required"))
	}
	data, err := p.get(ctx, "/incidents/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOncalls(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userIDs := r.Str("user_ids")
	policyIDs := r.Str("escalation_policy_ids")
	scheduleIDs := r.Str("schedule_ids")
	since := r.Str("since")
	until := r.Str("until")
	limit := r.OptInt("limit", defaultLimit)
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	vals := paginationQuery(limit, offset)
	addCSV(vals, "user_ids[]", userIDs)
	addCSV(vals, "escalation_policy_ids[]", policyIDs)
	addCSV(vals, "schedule_ids[]", scheduleIDs)
	if since != "" {
		vals.Set("since", since)
	}
	if until != "" {
		vals.Set("until", until)
	}
	data, err := p.get(ctx, "/oncalls?%s", vals.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listServices(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	limit := r.OptInt("limit", defaultLimit)
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"query":  query,
		"limit":  strconv.Itoa(clampLimit(limit)),
		"offset": strconv.Itoa(offset),
	}
	data, err := p.get(ctx, "/services%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func resolveFrom(p *pagerduty, from string) (string, error) {
	if from == "" {
		from = p.fromEmail
	}
	if from == "" {
		return "", fmt.Errorf("from_email is required for write operations")
	}
	return from, nil
}

func addIncidentNote(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("incident_id")
	content := r.Str("content")
	from := r.Str("from_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("incident_id is required"))
	}
	if content == "" {
		return mcp.ErrResult(fmt.Errorf("content is required"))
	}
	from, err := resolveFrom(p, from)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"note": map[string]any{"content": content}}
	data, err := p.post(ctx, "/incidents/"+url.PathEscape(id)+"/notes", from, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateIncidentStatus(ctx context.Context, p *pagerduty, args map[string]any, status string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("incident_id")
	from := r.Str("from_email")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("incident_id is required"))
	}
	from, err := resolveFrom(p, from)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"incident": map[string]any{
			"type":   "incident",
			"status": status,
		},
	}
	data, err := p.put(ctx, "/incidents/"+url.PathEscape(id), from, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func acknowledgeIncident(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error) {
	return updateIncidentStatus(ctx, p, args, "acknowledged")
}

func resolveIncident(ctx context.Context, p *pagerduty, args map[string]any) (*mcp.ToolResult, error) {
	return updateIncidentStatus(ctx, p, args, "resolved")
}
