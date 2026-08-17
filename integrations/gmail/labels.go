package gmail

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listLabels(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/labels", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	labelID := r.Str("label_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/labels/%s", u, labelID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"name": r.Str("name"),
	}
	if v := r.Str("message_list_visibility"); v != "" {
		body["messageListVisibility"] = v
	}
	if v := r.Str("label_list_visibility"); v != "" {
		body["labelListVisibility"] = v
	}
	if bg := r.Str("background_color"); bg != "" || r.Str("text_color") != "" {
		color := map[string]string{}
		if bg != "" {
			color["backgroundColor"] = bg
		}
		if tc := r.Str("text_color"); tc != "" {
			color["textColor"] = tc
		}
		body["color"] = color
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/labels", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"id": r.Str("label_id"),
	}
	if v := r.Str("name"); v != "" {
		body["name"] = v
	}
	if v := r.Str("message_list_visibility"); v != "" {
		body["messageListVisibility"] = v
	}
	if v := r.Str("label_list_visibility"); v != "" {
		body["labelListVisibility"] = v
	}
	if bg := r.Str("background_color"); bg != "" || r.Str("text_color") != "" {
		color := map[string]string{}
		if bg != "" {
			color["backgroundColor"] = bg
		}
		if tc := r.Str("text_color"); tc != "" {
			color["textColor"] = tc
		}
		body["color"] = color
	}
	u := user(r)
	labelID := r.Str("label_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/labels/%s", u, labelID)
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	labelID := r.Str("label_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/labels/%s", u, labelID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
