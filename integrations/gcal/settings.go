package gcal

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func listSettings(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"maxResults": r.Str("max_results"),
		"pageToken":  r.Str("page_token"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/users/me/settings%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSetting(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sid := r.Str("setting_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/users/me/settings/%s", pathEscape(sid))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getColors(ctx context.Context, g *gcal, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/colors")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
