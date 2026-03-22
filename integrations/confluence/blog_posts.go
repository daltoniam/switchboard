package confluence

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listBlogPosts(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Str("space_id"); v != "" {
		params["space-id"] = v
	}
	if v := r.Str("title"); v != "" {
		params["title"] = v
	}
	if v := r.Str("status"); v != "" {
		params["status"] = v
	}
	if v := r.Str("cursor"); v != "" {
		params["cursor"] = v
	}
	if v := r.Int("limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("sort"); v != "" {
		params["sort"] = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "/blogposts%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	bodyFormat := r.Str("body_format")
	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	params["body-format"] = bodyFormat
	if v := r.Int("version"); v > 0 {
		params["version"] = fmt.Sprintf("%d", v)
	}
	blogpostID := r.Str("blogpost_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "/blogposts/%s%s", url.PathEscape(blogpostID), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bodyFormat := r.Str("body_format")
	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	body := map[string]any{
		"spaceId": r.Str("space_id"),
		"title":   r.Str("title"),
		"body": map[string]any{
			"representation": bodyFormat,
			"value":          r.Str("body_value"),
		},
	}
	if v := r.Str("status"); v != "" {
		body["status"] = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.post(ctx, "/blogposts", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bodyFormat := r.Str("body_format")
	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	version := map[string]any{
		"number": r.Int("version_number"),
	}
	if v := r.Str("version_message"); v != "" {
		version["message"] = v
	}
	body := map[string]any{
		"id":      r.Str("blogpost_id"),
		"title":   r.Str("title"),
		"version": version,
		"body": map[string]any{
			"representation": bodyFormat,
			"value":          r.Str("body_value"),
		},
	}
	if v := r.Str("status"); v != "" {
		body["status"] = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/blogposts/%s", url.PathEscape(r.Str("blogpost_id")))
	data, err := c.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteBlogPost(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	blogpostID := r.Str("blogpost_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/blogposts/%s", url.PathEscape(blogpostID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
