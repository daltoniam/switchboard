package datadog

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	mcp "github.com/daltoniam/switchboard"
)

// ── Teams ────────────────────────────────────────────────────────────

func listTeams(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	api := datadogV2.NewTeamsApi(d.client)
	opts := datadogV2.NewListTeamsOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	if v := r.Str("filter"); v != "" {
		opts = opts.WithFilterKeyword(v)
	}
	if v := r.Str("sort"); v != "" {
		opts = opts.WithSort(datadogV2.ListTeamsSort(v))
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.ListTeams(ctx, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.GetTeam(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	handle := r.Str("handle")
	attrs := datadogV2.TeamCreateAttributes{
		Name:   name,
		Handle: handle,
	}
	if v := r.Str("description"); v != "" {
		attrs.Description = &v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.TeamCreateRequest{
		Data: datadogV2.TeamCreate{
			Attributes: attrs,
			Type:       datadogV2.TEAMTYPE_TEAM,
		},
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.CreateTeam(ctx, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("team_id")
	name := r.Str("name")
	handle := r.Str("handle")
	attrs := datadogV2.TeamUpdateAttributes{
		Name:   name,
		Handle: handle,
	}
	if v := r.Str("description"); v != "" {
		attrs.Description = &v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.TeamUpdateRequest{
		Data: datadogV2.TeamUpdate{
			Attributes: attrs,
			Type:       datadogV2.TEAMTYPE_TEAM,
		},
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.UpdateTeam(ctx, id, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteTeam(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	_, err := api.DeleteTeam(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Team Members ─────────────────────────────────────────────────────

func listTeamMembers(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	api := datadogV2.NewTeamsApi(d.client)
	opts := datadogV2.NewGetTeamMembershipsOptionalParameters()
	if v := r.Int64("page_size"); v > 0 {
		opts = opts.WithPageSize(v)
	}
	if v := r.Int64("page_number"); v > 0 {
		opts = opts.WithPageNumber(v)
	}
	if v := r.Str("filter"); v != "" {
		opts = opts.WithFilterKeyword(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	resp, _, err := api.GetTeamMemberships(ctx, teamID, *opts)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func addTeamMember(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	userID := r.Str("user_id")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	create := datadogV2.UserTeamCreate{
		Type: datadogV2.USERTEAMTYPE_TEAM_MEMBERSHIPS,
	}
	if role != "" {
		attrs := datadogV2.UserTeamAttributes{}
		r := datadogV2.UserTeamRole(role)
		nr := datadogV2.NewNullableUserTeamRole(&r)
		attrs.Role = *nr
		create.Attributes = &attrs
	}

	rel := datadogV2.UserTeamRelationships{
		User: &datadogV2.RelationshipToUserTeamUser{
			Data: datadogV2.RelationshipToUserTeamUserData{
				Id:   userID,
				Type: datadogV2.USERTEAMUSERTYPE_USERS,
			},
		},
	}
	create.Relationships = &rel

	body := datadogV2.UserTeamRequest{Data: create}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.CreateTeamMembership(ctx, teamID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateTeamMember(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	userID := r.Str("user_id")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	update := datadogV2.UserTeamUpdate{
		Type: datadogV2.USERTEAMTYPE_TEAM_MEMBERSHIPS,
	}
	if role != "" {
		attrs := datadogV2.UserTeamAttributes{}
		ur := datadogV2.UserTeamRole(role)
		nr := datadogV2.NewNullableUserTeamRole(&ur)
		attrs.Role = *nr
		update.Attributes = &attrs
	}

	body := datadogV2.UserTeamUpdateRequest{Data: update}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.UpdateTeamMembership(ctx, teamID, userID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func removeTeamMember(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	_, err := api.DeleteTeamMembership(ctx, teamID, userID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

func getUserTeamMemberships(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.GetUserMemberships(ctx, userID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

// ── Team Links ───────────────────────────────────────────────────────

func listTeamLinks(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.GetTeamLinks(ctx, teamID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func getTeamLink(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	linkID := r.Str("link_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.GetTeamLink(ctx, teamID, linkID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func createTeamLink(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	label := r.Str("label")
	url := r.Str("url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.TeamLinkCreateRequest{
		Data: datadogV2.TeamLinkCreate{
			Attributes: datadogV2.TeamLinkAttributes{
				Label: label,
				Url:   url,
			},
			Type: datadogV2.TEAMLINKTYPE_TEAM_LINKS,
		},
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.CreateTeamLink(ctx, teamID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateTeamLink(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	linkID := r.Str("link_id")
	label := r.Str("label")
	url := r.Str("url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := datadogV2.TeamLinkCreateRequest{
		Data: datadogV2.TeamLinkCreate{
			Attributes: datadogV2.TeamLinkAttributes{
				Label: label,
				Url:   url,
			},
			Type: datadogV2.TEAMLINKTYPE_TEAM_LINKS,
		},
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.UpdateTeamLink(ctx, teamID, linkID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func deleteTeamLink(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	linkID := r.Str("link_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	_, err := api.DeleteTeamLink(ctx, teamID, linkID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Team Permission Settings ─────────────────────────────────────────

func getTeamPermissionSettings(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.GetTeamPermissionSettings(ctx, teamID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}

func updateTeamPermissionSetting(ctx context.Context, d *dd, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	action := r.Str("action")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	v := datadogV2.TeamPermissionSettingValue(value)
	body := datadogV2.TeamPermissionSettingUpdateRequest{
		Data: datadogV2.TeamPermissionSettingUpdate{
			Attributes: &datadogV2.TeamPermissionSettingUpdateAttributes{
				Value: &v,
			},
			Type: datadogV2.TEAMPERMISSIONSETTINGTYPE_TEAM_PERMISSION_SETTINGS,
		},
	}
	api := datadogV2.NewTeamsApi(d.client)
	resp, _, err := api.UpdateTeamPermissionSetting(ctx, teamID, action, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(resp)
}
