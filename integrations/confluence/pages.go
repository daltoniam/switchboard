package confluence

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listPages(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argStr(args, "space_id"); v != "" {
		params["space-id"] = v
	}
	if v := argStr(args, "title"); v != "" {
		params["title"] = v
	}
	if v := argStr(args, "status"); v != "" {
		params["status"] = v
	}
	if v := argStr(args, "cursor"); v != "" {
		params["cursor"] = v
	}
	if v := argInt(args, "limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "sort"); v != "" {
		params["sort"] = v
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "/pages%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPage(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	bodyFormat := argStr(args, "body_format")
	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	params["body-format"] = bodyFormat
	if v := argInt(args, "version"); v > 0 {
		params["version"] = fmt.Sprintf("%d", v)
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "/pages/%s%s", url.PathEscape(argStr(args, "page_id")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPage(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	bodyFormat := argStr(args, "body_format")
	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	body := map[string]any{
		"spaceId": argStr(args, "space_id"),
		"title":   argStr(args, "title"),
		"body": map[string]any{
			"representation": bodyFormat,
			"value":          argStr(args, "body_value"),
		},
	}
	if v := argStr(args, "status"); v != "" {
		body["status"] = v
	}
	if v := argStr(args, "parent_id"); v != "" {
		body["parentId"] = v
	}
	data, err := c.post(ctx, "/pages", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePage(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	bodyFormat := argStr(args, "body_format")
	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	version := map[string]any{
		"number": argInt(args, "version_number"),
	}
	if v := argStr(args, "version_message"); v != "" {
		version["message"] = v
	}
	body := map[string]any{
		"id":      argStr(args, "page_id"),
		"title":   argStr(args, "title"),
		"version": version,
		"body": map[string]any{
			"representation": bodyFormat,
			"value":          argStr(args, "body_value"),
		},
	}
	if v := argStr(args, "status"); v != "" {
		body["status"] = v
	}
	path := fmt.Sprintf("/pages/%s", url.PathEscape(argStr(args, "page_id")))
	data, err := c.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deletePage(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	data, err := c.del(ctx, "/pages/%s", url.PathEscape(argStr(args, "page_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPageChildren(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argStr(args, "cursor"); v != "" {
		params["cursor"] = v
	}
	if v := argInt(args, "limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "sort"); v != "" {
		params["sort"] = v
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "/pages/%s/children%s", url.PathEscape(argStr(args, "page_id")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
