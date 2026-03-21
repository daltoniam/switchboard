package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listStates(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.get(ctx, "/api/states")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getState(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	entityID := r.Str("entity_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if entityID == "" {
		return mcp.ErrResult(fmt.Errorf("entity_id is required"))
	}
	data, err := h.get(ctx, "/api/states/%s", url.PathEscape(entityID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func setState(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	entityID := r.Str("entity_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if entityID == "" {
		return mcp.ErrResult(fmt.Errorf("entity_id is required"))
	}
	body := map[string]any{
		"state": r.Str("state"),
	}
	if v := r.Str("attributes"); v != "" {
		var attrs map[string]any
		if err := json.Unmarshal([]byte(v), &attrs); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for attributes: %w", err))
		}
		body["attributes"] = attrs
	}
	data, err := h.post(ctx, fmt.Sprintf("/api/states/%s", url.PathEscape(entityID)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteState(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	entityID := r.Str("entity_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if entityID == "" {
		return mcp.ErrResult(fmt.Errorf("entity_id is required"))
	}
	data, err := h.del(ctx, "/api/states/%s", url.PathEscape(entityID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
