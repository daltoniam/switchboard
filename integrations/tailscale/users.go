package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func userPath(id, suffix string) string {
	return "/users/" + url.PathEscape(id) + suffix
}

func listUsers(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userType := r.Str("type")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	vals := url.Values{}
	if userType != "" {
		vals.Set("type", userType)
	}
	if role != "" {
		vals.Set("role", role)
	}
	path := t.tailnetPath("/users")
	if len(vals) > 0 {
		path += "?" + vals.Encode()
	}
	return rawResult(t.doJSON(ctx, http.MethodGet, path, nil))
}

func getUser(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return userAction(ctx, t, args, http.MethodGet, "")
}

func approveUser(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return userAction(ctx, t, args, http.MethodPost, "/approve")
}

func suspendUser(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return userAction(ctx, t, args, http.MethodPost, "/suspend")
}

func restoreUser(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return userAction(ctx, t, args, http.MethodPost, "/restore")
}

func deleteUser(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	return userAction(ctx, t, args, http.MethodPost, "/delete")
}

func userAction(ctx context.Context, t *tailscale, args map[string]any, method, suffix string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("user_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, method, userPath(id, suffix), nil))
}

func updateUserRole(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("user_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("role", role); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, userPath(id, "/role"), map[string]any{"role": role}))
}

// listUserInvites always returns {"invites":[...]}. The documented response
// is a bare array; wrapping it gives compaction a stable envelope, matches the
// other list tools, and also passes an already-wrapped response through.
func listUserInvites(ctx context.Context, t *tailscale, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := t.doJSON(ctx, http.MethodGet, t.tailnetPath("/user-invites"), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return mcp.ErrResult(fmt.Errorf("tailscale: decode user invites response: %w", err))
	}
	switch v := raw.(type) {
	case []any:
		return mcp.JSONResult(map[string]any{"invites": v})
	case nil:
		return mcp.JSONResult(map[string]any{"invites": []any{}})
	case map[string]any:
		if _, ok := v["invites"]; ok {
			return mcp.JSONResult(v)
		}
	}
	return mcp.ErrResult(fmt.Errorf("tailscale: unexpected user invites response shape"))
}

func createUserInvite(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	email := r.Str("email")
	role := r.Str("role")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	invite := map[string]any{}
	if email != "" {
		invite["email"] = email
	}
	if role != "" {
		invite["role"] = role
	}
	return rawResult(t.doJSON(ctx, http.MethodPost, t.tailnetPath("/user-invites"), []map[string]any{invite}))
}

func deleteUserInvite(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("invite_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireStr("invite_id", id); err != nil {
		return mcp.ErrResult(err)
	}
	return rawResult(t.doJSON(ctx, http.MethodDelete, "/user-invites/"+url.PathEscape(id), nil))
}
