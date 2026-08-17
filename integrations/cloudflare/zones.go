package cloudflare

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listZones(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	status := r.Str("status")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"name":     name,
		"status":   status,
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/zones%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getZone(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/zones/%s", zoneID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createZone(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	typ := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	acct, err := c.acctID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"name": name,
		"account": map[string]any{
			"id": acct,
		},
	}
	if typ != "" {
		body["type"] = typ
	}
	if v, _ := mcp.ArgBool(args, "jump_start"); v {
		body["jump_start"] = true
	}
	data, err := c.post(ctx, "/zones", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func editZone(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if _, ok := args["paused"]; ok {
		v, err := mcp.ArgBool(args, "paused")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["paused"] = v
	}
	if v, _ := mcp.ArgStr(args, "plan"); v != "" {
		body["plan"] = map[string]any{"id": v}
	}
	path := fmt.Sprintf("/zones/%s", zoneID)
	data, err := c.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteZone(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/zones/%s", zoneID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func purgeCache(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if v, _ := mcp.ArgBool(args, "purge_everything"); v {
		body["purge_everything"] = true
	}
	if v, _ := mcp.ArgStrSlice(args, "files"); len(v) > 0 {
		body["files"] = v
	}
	if v, _ := mcp.ArgStr(args, "tags"); v != "" {
		body["tags"] = splitCSV(v)
	}
	if v, _ := mcp.ArgStr(args, "hosts"); v != "" {
		body["hosts"] = splitCSV(v)
	}
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("purge_cache: at least one of purge_everything, files, tags, or hosts is required"))
	}
	path := fmt.Sprintf("/zones/%s/purge_cache", zoneID)
	data, err := c.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
