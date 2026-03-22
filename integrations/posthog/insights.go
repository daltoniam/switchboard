package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listInsights(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"search":     r.Str("search"),
		"created_by": r.Str("created_by"),
		"limit":      r.Str("limit"),
		"offset":     r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/insights/%s", p.proj(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	insightID, err := mcp.ArgStr(args, "insight_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/insights/%s/", p.proj(args), insightID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	if query, err := parseJSON(args, "query"); err != nil {
		return mcp.ErrResult(err)
	} else if query != nil {
		body["query"] = query
	}
	path := fmt.Sprintf("/api/projects/%s/insights/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	insightID := r.Str("insight_id")
	name := r.Str("name")
	description := r.Str("description")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	if query, err := parseJSON(args, "query"); err != nil {
		return mcp.ErrResult(err)
	} else if query != nil {
		body["query"] = query
	}
	path := fmt.Sprintf("/api/projects/%s/insights/%s/", p.proj(args), insightID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	insightID, err := mcp.ArgStr(args, "insight_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/insights/%s/", p.proj(args), insightID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
