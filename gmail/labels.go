package gmail

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listLabels(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/labels", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/labels/%s", user(args), argStr(args, "label_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"name": argStr(args, "name"),
	}
	if v := argStr(args, "message_list_visibility"); v != "" {
		body["messageListVisibility"] = v
	}
	if v := argStr(args, "label_list_visibility"); v != "" {
		body["labelListVisibility"] = v
	}
	if bg := argStr(args, "background_color"); bg != "" || argStr(args, "text_color") != "" {
		color := map[string]string{}
		if bg != "" {
			color["backgroundColor"] = bg
		}
		if tc := argStr(args, "text_color"); tc != "" {
			color["textColor"] = tc
		}
		body["color"] = color
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/labels", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"id": argStr(args, "label_id"),
	}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "message_list_visibility"); v != "" {
		body["messageListVisibility"] = v
	}
	if v := argStr(args, "label_list_visibility"); v != "" {
		body["labelListVisibility"] = v
	}
	if bg := argStr(args, "background_color"); bg != "" || argStr(args, "text_color") != "" {
		color := map[string]string{}
		if bg != "" {
			color["backgroundColor"] = bg
		}
		if tc := argStr(args, "text_color"); tc != "" {
			color["textColor"] = tc
		}
		body["color"] = color
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/labels/%s", user(args), argStr(args, "label_id"))
	data, err := g.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteLabel(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/labels/%s", user(args), argStr(args, "label_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
