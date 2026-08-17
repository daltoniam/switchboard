package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Stream videos ---

func listStreamVideos(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	after, _ := mcp.ArgStr(args, "after")
	before, _ := mcp.ArgStr(args, "before")
	creator, _ := mcp.ArgStr(args, "creator")
	status, _ := mcp.ArgStr(args, "status")
	search, _ := mcp.ArgStr(args, "search")
	q := queryEncode(map[string]string{
		"after":   after,
		"before":  before,
		"creator": creator,
		"status":  status,
		"search":  search,
	})
	data, err := c.get(ctx, "/accounts/%s/stream%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getStreamVideo(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	identifier := r.Str("identifier")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/stream/%s", acct, identifier)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteStreamVideo(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	identifier := r.Str("identifier")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/stream/%s", acct, identifier)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Cloudflare Images v1 ---

func listImages(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 50)),
	})
	data, err := c.get(ctx, "/accounts/%s/images/v1%s", acct, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getImage(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	imageID := r.Str("image_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/accounts/%s/images/v1/%s", acct, imageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteImage(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	imageID := r.Str("image_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/accounts/%s/images/v1/%s", acct, imageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
