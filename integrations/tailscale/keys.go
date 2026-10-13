package tailscale

import (
	"context"
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func keyPath(t *tailscale, id string) string {
	return t.tailnetPath("/keys/" + url.PathEscape(id))
}

func listKeys(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	all := r.Bool("all")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := t.tailnetPath("/keys")
	if all {
		path += "?all=true"
	}
	return rawResult(t.doJSON(ctx, http.MethodGet, path, nil))
}

func getKey(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return keyAction(ctx, t, args, http.MethodGet)
}

func deleteKey(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return keyAction(ctx, t, args, http.MethodDelete)
}

func keyAction(ctx context.Context, t *tailscale, args map[string]any, method string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("key_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("key_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, method, keyPath(t, id), nil))
}

func createAuthKey(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	reusable := r.Bool("reusable")
	ephemeral := r.Bool("ephemeral")
	preauthorized := r.Bool("preauthorized")
	tags := r.StrSlice("tags")
	expirySeconds := r.Int("expiry_seconds")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	create := map[string]any{
		"reusable":      reusable,
		"ephemeral":     ephemeral,
		"preauthorized": preauthorized,
	}
	if len(tags) > 0 {
		create["tags"] = tags
	}
	body := map[string]any{
		"keyType":      "auth",
		"capabilities": map[string]any{"devices": map[string]any{"create": create}},
	}
	if expirySeconds > 0 {
		body["expirySeconds"] = expirySeconds
	}
	if description != "" {
		body["description"] = description
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, t.tailnetPath("/keys"), body))
}
