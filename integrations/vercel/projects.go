package vercel

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID, teamSlug := optionalScopeArgs(r)
	params := paginationArgs(r, 20)
	if search := r.Str("search"); search != "" {
		params["search"] = search
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params["teamId"] = teamID
	params["slug"] = teamSlug
	data, err := v.get(ctx, "/v10/projects%s", queryEncode(v.scopedQuery(params)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getProject(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	idOrName := r.Str("id_or_name")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(idOrName, "id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.get(ctx, "/v10/projects/%s%s", url.PathEscape(idOrName), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProject(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID, teamSlug := optionalScopeArgs(r)
	body := r.Map("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if body == nil {
		return mcp.ErrResult(required("", "body"))
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.post(ctx, "/v9/projects"+queryEncode(params), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateProject(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	idOrName := r.Str("id_or_name")
	teamID, teamSlug := optionalScopeArgs(r)
	body := r.Map("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(idOrName, "id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	if body == nil {
		return mcp.ErrResult(required("", "body"))
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.patch(ctx, "/v9/projects/"+url.PathEscape(idOrName)+queryEncode(params), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteProject(ctx context.Context, v *vercel, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	idOrName := r.Str("id_or_name")
	teamID, teamSlug := optionalScopeArgs(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required(idOrName, "id_or_name"); err != nil {
		return mcp.ErrResult(err)
	}
	params := v.scopedQuery(map[string]string{"teamId": teamID, "slug": teamSlug})
	data, err := v.del(ctx, "/v9/projects/%s%s", url.PathEscape(idOrName), queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
