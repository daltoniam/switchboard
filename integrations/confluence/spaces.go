package confluence

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listSpaces(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Str("cursor"); v != "" {
		params["cursor"] = v
	}
	if v := r.Int("limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("type"); v != "" {
		params["type"] = v
	}
	if v := r.Str("status"); v != "" {
		params["status"] = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "/spaces%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSpace(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/spaces/%s", url.PathEscape(spaceID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func search(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cql := r.Str("cql")
	if cql == "" {
		return mcp.ErrResult(fmt.Errorf("cql is required"))
	}
	params := map[string]string{
		"cql": cql,
	}
	if v := r.Int("limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("start"); v > 0 {
		params["start"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("excerpt"); v != "" {
		params["excerpt"] = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	// CQL search uses v1 API since v2 doesn't support CQL
	data, err := c.v1Get(ctx, "/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
