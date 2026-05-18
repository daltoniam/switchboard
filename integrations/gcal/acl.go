package gcal

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listACL(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	params := map[string]string{
		"maxResults":  r.Str("max_results"),
		"pageToken":   r.Str("page_token"),
		"showDeleted": r.Str("show_deleted"),
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/calendars/%s/acl%s", pathEscape(cid), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getACL(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	rid := r.Str("rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/calendars/%s/acl/%s", pathEscape(cid), pathEscape(rid))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func buildACLBody(r *mcp.Args) (map[string]any, error) {
	if raw := r.Str("body"); raw != "" {
		var out map[string]any
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON for body: %w", err)
		}
		return out, nil
	}
	body := map[string]any{}
	if v := r.Str("role"); v != "" {
		body["role"] = v
	}
	scopeType := r.Str("scope_type")
	scopeValue := r.Str("scope_value")
	if scopeType != "" {
		scope := map[string]any{"type": scopeType}
		if scopeValue != "" {
			scope["value"] = scopeValue
		}
		body["scope"] = scope
	}
	return body, nil
}

func createACL(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	sendNotifs := r.Str("send_notifications")
	body, berr := buildACLBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"sendNotifications": sendNotifs})
	path := fmt.Sprintf("/calendars/%s/acl%s", pathEscape(cid), q)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateACL(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	rid := r.Str("rule_id")
	body, berr := buildACLBody(r)
	if berr != nil {
		return mcp.ErrResult(berr)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/calendars/%s/acl/%s", pathEscape(cid), pathEscape(rid))
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteACL(ctx context.Context, g *gcal, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cid := calendarID(r)
	rid := r.Str("rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/calendars/%s/acl/%s", pathEscape(cid), pathEscape(rid))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
