package vercel

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listTeams(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := paginationArgs(r, 20)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := v.get(ctx, "/v2/teams%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTeam(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" && teamSlug == "" {
		teamID = v.teamID
		teamSlug = v.teamSlug
	}
	if teamID == "" && teamSlug == "" {
		return mcp.ErrResult(fmt.Errorf("team_id or team_slug is required"))
	}
	if teamID != "" {
		data, err := v.get(ctx, "/v2/teams/%s", url.PathEscape(teamID))
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
	data, err := v.get(ctx, "/v2/teams/%s", url.PathEscape(teamSlug))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTeamMembers(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID, teamSlug := optionalScopeArgs(r)
	params := paginationArgs(r, 20)
	if search := r.Str("search"); search != "" {
		params["search"] = search
	}
	if role := r.Str("role"); role != "" {
		params["role"] = role
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params["teamId"] = teamID
	params["slug"] = teamSlug
	params = v.scopedQuery(params)
	if params["teamId"] == "" && params["slug"] == "" {
		return mcp.ErrResult(fmt.Errorf("team_id or team_slug is required"))
	}
	if params["teamId"] != "" {
		data, err := v.get(ctx, "/v3/teams/%s/members%s", url.PathEscape(params["teamId"]), queryEncode(params))
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
	data, err := v.get(ctx, "/v3/teams/%s/members%s", url.PathEscape(params["slug"]), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUserEvents(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID, teamSlug := optionalScopeArgs(r)
	params := paginationArgs(r, 20)
	if types := r.Str("types"); types != "" {
		params["types"] = types
	}
	if projectIDs := r.Str("project_ids"); projectIDs != "" {
		params["projectIds"] = projectIDs
	}
	if principalID := r.Str("principal_id"); principalID != "" {
		params["principalId"] = principalID
	}
	if r.Bool("with_payload") {
		params["withPayload"] = "true"
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params["teamId"] = teamID
	params["slug"] = teamSlug
	data, err := v.get(ctx, "/v3/events%s", queryEncode(v.scopedQuery(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
