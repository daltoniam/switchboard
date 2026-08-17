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
	r := mcp.NewArgs(args)
	parentType := r.Str("parent_type")
	parentID := r.Str("parent_id")
	params := map[string]string{}
	if v := r.Str("cursor"); v != "" {
		params["cursor"] = v
	}
	if v := r.Int("limit"); v > 0 {
		params["limit"] = fmt.Sprintf("%d", v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	basePath := commentBasePath(parentType, url.PathEscape(parentID))
	q := queryEncode(params)
	data, err := c.get(ctx, "%s%s", basePath, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createComment(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parentType := r.Str("parent_type")
	parentID := r.Str("parent_id")
	bodyFormat := r.Str("body_format")
	bodyValue := r.Str("body_value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	basePath := commentBasePath(parentType, url.PathEscape(parentID))

	if bodyFormat == "" {
		bodyFormat = "storage"
	}
	body := map[string]any{
		"body": map[string]any{
			"representation": bodyFormat,
			"value":          bodyValue,
		},
	}
	data, err := c.post(ctx, basePath, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteComment(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	commentID := r.Str("comment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/footer-comments/%s", url.PathEscape(commentID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
