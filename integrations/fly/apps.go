package fly

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listApps(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	orgSlug := argStr(args, "org_slug")
	data, err := f.get(ctx, "/apps?org_slug=%s", orgSlug)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getApp(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	data, err := f.get(ctx, "/apps/%s", app)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createApp(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"app_name": argStr(args, "app_name"),
		"org_slug": argStr(args, "org_slug"),
	}
	if v := argStr(args, "network"); v != "" {
		body["network"] = v
	}
	data, err := f.post(ctx, "/apps", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteApp(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	params := map[string]string{}
	if argBool(args, "force") {
		params["force"] = "true"
	}
	path := fmt.Sprintf("/apps/%s", app)
	data, err := f.delWithQuery(ctx, path, params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
