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
	return mcp.RawResult(unwrapData(data))
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
	return mcp.RawResult(unwrapData(data))
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
		"title":       title,
		"description": description,
		"tags":        tags,
		"layout":      []any{},
		"widgets":     []any{},
		"variables":   map[string]any{},
		"version":     "v5",
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

	var content map[string]any
	switch v := dashRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &content); err != nil {
			return mcp.ErrResult(fmt.Errorf("dashboard must be valid JSON: %w", err))
		}
	case map[string]any:
		content = v
	default:
		return mcp.ErrResult(fmt.Errorf("dashboard must be a JSON object"))
	}

	content = extractDashboardContent(content)

	data, err := s.put(ctx, fmt.Sprintf("/api/v1/dashboards/%s", url.PathEscape(id)), content)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// extractDashboardContent unwraps a dashboard object down to the content level.
// The SigNoz PUT endpoint wraps the body in one layer automatically, so we must
// send just the content (title, widgets, layout, etc.) — not the full GET response.
//
// Handles these cases:
//   - Full GET response: {id, createdAt, data: {title, widgets...}} → {title, widgets...}
//   - Already content:   {title, widgets, layout...}                → returned as-is
func extractDashboardContent(obj map[string]any) map[string]any {
	if _, hasTitle := obj["title"]; hasTitle {
		return obj
	}
	if content := extractInnerMap(obj, "data"); content != nil {
		return content
	}
	return obj
}

// extractInnerMap walks obj["data"] (and obj["data"]["data"]) looking for a map
// containing a "title" key — the dashboard content level.
func extractInnerMap(obj map[string]any, key string) map[string]any {
	inner, ok := obj[key].(map[string]any)
	if !ok {
		return nil
	}
	if _, hasTitle := inner["title"]; hasTitle {
		return inner
	}
	if deeper, ok := inner["data"].(map[string]any); ok {
		if _, hasTitle := deeper["title"]; hasTitle {
			return deeper
		}
	}
	return nil
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
