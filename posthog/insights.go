package posthog

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listInsights(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"search":     argStr(args, "search"),
		"created_by": argStr(args, "created_by"),
		"limit":      argStr(args, "limit"),
		"offset":     argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/insights/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/insights/%s/", p.proj(args), argStr(args, "insight_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "filters"); v != "" {
		var filters any
		if err := json.Unmarshal([]byte(v), &filters); err == nil {
			body["filters"] = filters
		}
	}
	if v := argStr(args, "query"); v != "" {
		var query any
		if err := json.Unmarshal([]byte(v), &query); err == nil {
			body["query"] = query
		}
	}
	path := fmt.Sprintf("/api/projects/%s/insights/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "filters"); v != "" {
		var filters any
		if err := json.Unmarshal([]byte(v), &filters); err == nil {
			body["filters"] = filters
		}
	}
	if v := argStr(args, "query"); v != "" {
		var query any
		if err := json.Unmarshal([]byte(v), &query); err == nil {
			body["query"] = query
		}
	}
	path := fmt.Sprintf("/api/projects/%s/insights/%s/", p.proj(args), argStr(args, "insight_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/api/projects/%s/insights/%s/", p.proj(args), argStr(args, "insight_id"))
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
