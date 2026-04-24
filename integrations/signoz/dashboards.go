package signoz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listDashboards(ctx context.Context, s *signoz, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/api/v1/dashboards")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDashboard(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/dashboards/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDashboard(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	description, _ := mcp.ArgStr(args, "description")
	tagsStr, _ := mcp.ArgStr(args, "tags")

	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	body := map[string]any{
		"data": map[string]any{
			"title":       title,
			"description": description,
			"tags":        tags,
			"layout":      []any{},
			"widgets":     []any{},
			"variables":   map[string]any{},
		},
	}

	data, err := s.post(ctx, "/api/v1/dashboards", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDashboard(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	dashRaw, ok := args["dashboard"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("dashboard parameter is required"))
	}

	var body any
	switch v := dashRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("dashboard must be valid JSON: %w", err))
		}
	default:
		body = v
	}

	data, err := s.put(ctx, fmt.Sprintf("/api/v1/dashboards/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDashboard(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/api/v1/dashboards/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
