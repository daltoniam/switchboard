package confluence

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func commentBasePath(parentType, parentID string) string {
	switch parentType {
	case "blogpost":
		return fmt.Sprintf("/blogposts/%s/footer-comments", parentID)
	default:
		return fmt.Sprintf("/pages/%s/footer-comments", parentID)
	}
}

func listComments(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	basePath := commentBasePath(argStr(args, "parent_type"), url.PathEscape(argStr(args, "parent_id")))

	params := map[string]string{}
	if v := argStr(args, "cursor"); v != "" {
		params["cursor"] = v
	}
	if v := argInt(args, "limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	q := queryEncode(params)
	data, err := c.get(ctx, "%s%s", basePath, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createComment(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	basePath := commentBasePath(argStr(args, "parent_type"), url.PathEscape(argStr(args, "parent_id")))

	bodyFormat := argStr(args, "body_format")
	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	body := map[string]any{
		"body": map[string]any{
			"representation": bodyFormat,
			"value":          argStr(args, "body_value"),
		},
	}
	data, err := c.post(ctx, basePath, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteComment(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	data, err := c.del(ctx, "/footer-comments/%s", url.PathEscape(argStr(args, "comment_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
