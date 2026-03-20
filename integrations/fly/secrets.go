package fly

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listSecrets(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	data, err := f.get(ctx, "/apps/%s/secrets", app)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func setSecrets(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	secrets := argMap(args, "secrets")
	if secrets == nil {
		return &mcp.ToolResult{Data: "secrets parameter is required", IsError: true}, nil
	}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/secrets", app), secrets)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func unsetSecrets(ctx context.Context, f *fly, args map[string]any) (*mcp.ToolResult, error) {
	app := argStr(args, "app_name")
	keys := argStrSlice(args, "keys")
	if len(keys) == 0 {
		return &mcp.ToolResult{Data: "keys parameter is required", IsError: true}, nil
	}
	body := map[string]any{"keys": keys}
	data, err := f.post(ctx, fmt.Sprintf("/apps/%s/secrets/unset", app), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
