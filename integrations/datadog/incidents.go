package datadog

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
)

// ── Incident Services (Deprecated) ──────────────────────────────────

func listIncidentServices(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewIncidentServicesApi(d.client)
	opts := datadogV2.NewListIncidentServicesOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	if v := r.Str("filter"); v != "" {
		opts = opts.WithFilter(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListIncidentServices(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getIncidentService(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("service_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentServicesApi(d.client)
	resp, _, err := api.GetIncidentService(ctx, id, *datadogV2.NewGetIncidentServiceOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createIncidentService(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.IncidentServiceCreateRequest{
		Data: datadogV2.IncidentServiceCreateData{
			Attributes: &datadogV2.IncidentServiceCreateAttributes{
				Name: name,
			},
			Type: datadogV2.INCIDENTSERVICETYPE_SERVICES,
		},
	}
	api := datadogV2.NewIncidentServicesApi(d.client)
	resp, _, err := api.CreateIncidentService(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateIncidentService(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("service_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.IncidentServiceUpdateRequest{
		Data: datadogV2.IncidentServiceUpdateData{
			Attributes: &datadogV2.IncidentServiceUpdateAttributes{
				Name: name,
			},
			Type: datadogV2.INCIDENTSERVICETYPE_SERVICES,
		},
	}
	api := datadogV2.NewIncidentServicesApi(d.client)
	resp, _, err := api.UpdateIncidentService(ctx, id, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteIncidentService(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("service_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentServicesApi(d.client)
	_, err := api.DeleteIncidentService(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Incident Teams (Deprecated) ─────────────────────────────────────

func listIncidentTeams(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewIncidentTeamsApi(d.client)
	opts := datadogV2.NewListIncidentTeamsOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	if v := r.Str("filter"); v != "" {
		opts = opts.WithFilter(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListIncidentTeams(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getIncidentTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentTeamsApi(d.client)
	resp, _, err := api.GetIncidentTeam(ctx, id, *datadogV2.NewGetIncidentTeamOptionalParameters())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createIncidentTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.IncidentTeamCreateRequest{
		Data: datadogV2.IncidentTeamCreateData{
			Attributes: &datadogV2.IncidentTeamCreateAttributes{
				Name: name,
			},
			Type: datadogV2.INCIDENTTEAMTYPE_TEAMS,
		},
	}
	api := datadogV2.NewIncidentTeamsApi(d.client)
	resp, _, err := api.CreateIncidentTeam(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateIncidentTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("team_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.IncidentTeamUpdateRequest{
		Data: datadogV2.IncidentTeamUpdateData{
			Attributes: &datadogV2.IncidentTeamUpdateAttributes{
				Name: name,
			},
			Type: datadogV2.INCIDENTTEAMTYPE_TEAMS,
		},
	}
	api := datadogV2.NewIncidentTeamsApi(d.client)
	resp, _, err := api.UpdateIncidentTeam(ctx, id, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteIncidentTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentTeamsApi(d.client)
	_, err := api.DeleteIncidentTeam(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Extended Incidents (search, attachments, todos) ──────────────────

func searchIncidents(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	api := datadogV2.NewIncidentsApi(d.client)
	opts := datadogV2.NewSearchIncidentsOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_offset"); v > 0 {
		opts = opts.WithPageOffset(v)
	}
	if v := r.Str("sort"); v != "" {
		opts = opts.WithSort(datadogV2.IncidentSearchSortOrder(v))
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.SearchIncidents(ctx, query, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func listIncidentAttachments(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	incidentID := r.Str("incident_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentsApi(d.client)
	opts := datadogV2.NewListIncidentAttachmentsOptionalParameters()
	resp, _, err := api.ListIncidentAttachments(ctx, incidentID, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func listIncidentTodos(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	incidentID := r.Str("incident_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewIncidentsApi(d.client)
	resp, _, err := api.ListIncidentTodos(ctx, incidentID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
