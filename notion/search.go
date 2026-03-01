package notion

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func searchNotion(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "query"); v != "" {
		body["query"] = v
	}
	if v := argMap(args, "filter"); v != nil {
		body["filter"] = v
	}
	if v := argMap(args, "sort"); v != nil {
		body["sort"] = v
	}
	if v := argStr(args, "start_cursor"); v != "" {
		body["start_cursor"] = v
	}
	if v := argInt(args, "page_size"); v > 0 {
		body["page_size"] = v
	}
	data, err := n.post(ctx, "/v1/search", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
