package fly

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listApps(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	orgSlug := r.Str("org_slug")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{"org_slug": orgSlug}
	data, err := f.get(ctx, "/apps%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getApp(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/apps/%s", url.PathEscape(app))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createApp(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appName := r.Str("app_name")
	orgSlug := r.Str("org_slug")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"app_name": appName,
		"org_slug": orgSlug,
	}
	network, err := mcp.ArgStr(args, "network")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if network != "" {
		body["network"] = network
	}
	data, err := f.post(ctx, "/apps", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteApp(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	force := r.Bool("force")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{}
	if force {
		params["force"] = "true"
	}
	path := fmt.Sprintf("/apps/%s", url.PathEscape(app))
	data, err := f.delWithQuery(ctx, path, params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
