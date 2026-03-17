package confluence

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listSpaces(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argStr(args, "cursor"); v != "" {
		params["cursor"] = v
	}
	if v := argInt(args, "limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "type"); v != "" {
		params["type"] = v
	}
	if v := argStr(args, "status"); v != "" {
		params["status"] = v
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "/spaces%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSpace(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	data, err := c.get(ctx, "/spaces/%s", url.PathEscape(argStr(args, "space_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func search(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	cql := argStr(args, "cql")
	if cql == "" {
		return mcp.ErrResult(fmt.Errorf("cql is required"))
	}
	params := map[string]string{
		"cql": cql,
	}
	if v := argInt(args, "limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "start"); v > 0 {
		params["start"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "excerpt"); v != "" {
		params["excerpt"] = v
	}
	q := queryEncode(params)
	// CQL search uses v1 API since v2 doesn't support CQL
	data, err := c.v1Get(ctx, "/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
