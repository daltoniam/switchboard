package notion

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func listUsers(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	vals := url.Values{}
	if v := argStr(args, "start_cursor"); v != "" {
		vals.Set("start_cursor", v)
	}
	if v := argInt(args, "page_size"); v > 0 {
		vals.Set("page_size", strconv.Itoa(v))
	}
	path := "/v1/users"
	if len(vals) > 0 {
		path += "?" + vals.Encode()
	}
	data, err := n.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func retrieveUser(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	userID := argStr(args, "user_id")
	if userID == "" {
		return errResult(fmt.Errorf("user_id is required"))
	}
	data, err := n.get(ctx, "/v1/users/%s", userID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getSelf(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	data, err := n.get(ctx, "/v1/users/me")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
