package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listInsights(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
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
	data, err := p.get(ctx, "/api/projects/%s/insights/%s", projID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	insightID, err := mcp.ArgStr(args, "insight_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/insights/%s/", projID, insightID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
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
	path := fmt.Sprintf("/api/projects/%s/insights/", projID)
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
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
	path := fmt.Sprintf("/api/projects/%s/insights/%s/", projID, insightID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteInsight(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	insightID, err := mcp.ArgStr(args, "insight_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/insights/%s/", projID, insightID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func runQuery(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	hogql := r.Str("query")
	clientQueryID := r.Str("client_query_id")
	refresh := r.Str("refresh")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if hogql == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}
	body := map[string]any{
		"query": map[string]any{
			"kind":  "HogQLQuery",
			"query": hogql,
		},
	}
	if clientQueryID != "" {
		body["client_query_id"] = clientQueryID
	}
	if refresh != "" {
		body["refresh"] = refresh
	}
	path := fmt.Sprintf("/api/projects/%s/query/", projID)
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
