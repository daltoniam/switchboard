package signoz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// --- Alert Rules ---

func listAlerts(ctx context.Context, s *signoz, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/api/v1/rules")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func getAlert(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/rules/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func createAlert(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	ruleRaw, ok := args["rule"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("rule parameter is required"))
	}
	var body any
	switch v := ruleRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("rule must be valid JSON: %w", err))
		}
	default:
		body = v
	}
	data, err := s.post(ctx, "/api/v1/rules", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateAlert(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ruleRaw, ok := args["rule"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("rule parameter is required"))
	}
	var body any
	switch v := ruleRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("rule must be valid JSON: %w", err))
		}
	default:
		body = v
	}
	data, err := s.put(ctx, fmt.Sprintf("/api/v1/rules/%s", url.PathEscape(id)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAlert(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/api/v1/rules/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Saved Views ---

func listSavedViews(ctx context.Context, s *signoz, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/api/v1/explorer/views")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func getSavedView(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	viewID := r.Str("view_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/api/v1/explorer/views/%s", url.PathEscape(viewID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

func createSavedView(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	viewRaw, ok := args["view"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("view parameter is required"))
	}
	var body any
	switch v := viewRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("view must be valid JSON: %w", err))
		}
	default:
		body = v
	}
	data, err := s.post(ctx, "/api/v1/explorer/views", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateSavedView(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	viewID := r.Str("view_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	viewRaw, ok := args["view"]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("view parameter is required"))
	}
	var body any
	switch v := viewRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &body); err != nil {
			return mcp.ErrResult(fmt.Errorf("view must be valid JSON: %w", err))
		}
	default:
		body = v
	}
	data, err := s.put(ctx, fmt.Sprintf("/api/v1/explorer/views/%s", url.PathEscape(viewID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteSavedView(ctx context.Context, s *signoz, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	viewID := r.Str("view_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/api/v1/explorer/views/%s", url.PathEscape(viewID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Notification Channels ---

func listChannels(ctx context.Context, s *signoz, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/api/v1/channels")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(unwrapData(data))
}

// --- Extras ---

func getVersion(ctx context.Context, s *signoz, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/api/v1/version")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
