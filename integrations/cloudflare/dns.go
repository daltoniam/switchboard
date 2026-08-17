package cloudflare

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDNSRecords(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	typ := r.Str("type")
	name := r.Str("name")
	content := r.Str("content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"type":     typ,
		"name":     name,
		"content":  content,
		"page":     fmt.Sprintf("%d", mcp.OptInt(args, "page", 1)),
		"per_page": fmt.Sprintf("%d", mcp.OptInt(args, "per_page", 20)),
	})
	data, err := c.get(ctx, "/zones/%s/dns_records%s", zoneID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDNSRecord(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	recordID := r.Str("record_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.get(ctx, "/zones/%s/dns_records/%s", zoneID, recordID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDNSRecord(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	typ := r.Str("type")
	name := r.Str("name")
	content := r.Str("content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"type":    typ,
		"name":    name,
		"content": content,
		"ttl":     mcp.OptInt(args, "ttl", 1),
	}
	if _, ok := args["proxied"]; ok {
		v, err := mcp.ArgBool(args, "proxied")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["proxied"] = v
	}
	if v := mcp.OptInt(args, "priority", 0); v > 0 {
		body["priority"] = v
	}
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)
	data, err := c.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDNSRecord(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	recordID := r.Str("record_id")
	typ := r.Str("type")
	name := r.Str("name")
	content := r.Str("content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"type":    typ,
		"name":    name,
		"content": content,
		"ttl":     mcp.OptInt(args, "ttl", 1),
	}
	if _, ok := args["proxied"]; ok {
		v, err := mcp.ArgBool(args, "proxied")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["proxied"] = v
	}
	if v := mcp.OptInt(args, "priority", 0); v > 0 {
		body["priority"] = v
	}
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)
	data, err := c.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDNSRecord(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	zoneID := r.Str("zone_id")
	recordID := r.Str("record_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := c.del(ctx, "/zones/%s/dns_records/%s", zoneID, recordID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
