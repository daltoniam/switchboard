package fly

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listSecrets(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := f.get(ctx, "/apps/%s/secrets", url.PathEscape(app))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func setSecrets(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	secrets, err := mcp.ArgMap(args, "secrets")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if secrets == nil {
		return &mcp.ToolResult{Data: "secrets parameter is required", IsError: true}, nil
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/secrets", url.PathEscape(app)), secrets)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unsetSecrets(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	app := r.Str("app_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	keys, err := mcp.ArgStrSlice(args, "keys")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(keys) == 0 {
		return &mcp.ToolResult{Data: "keys parameter is required", IsError: true}, nil
	}
	body := map[string]any{"keys": keys}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/secrets/unset", url.PathEscape(app)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
