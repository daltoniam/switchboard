package confluence

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listBlogPosts(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := c.get(ctx, "/blogposts%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := c.get(ctx, "/blogposts/%s%s", url.PathEscape(argStr(args, "blogpost_id")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
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
	data, err := c.post(ctx, "/blogposts", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
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
		"id":      argStr(args, "blogpost_id"),
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
	path := fmt.Sprintf("/blogposts/%s", url.PathEscape(argStr(args, "blogpost_id")))
	data, err := c.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	data, err := c.del(ctx, "/blogposts/%s", url.PathEscape(argStr(args, "blogpost_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
