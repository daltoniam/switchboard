package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listEvents(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.get(ctx, "/api/events")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func fireEvent(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	eventType := r.Str("event_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if eventType == "" {
		return mcp.ErrResult(fmt.Errorf("event_type is required"))
	}

	var body any
	if v := r.Str("event_data"); v != "" {
		var data map[string]any
		if err := json.Unmarshal([]byte(v), &data); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for event_data: %w", err))
		}
		body = data
	}

	result, err := h.post(ctx, fmt.Sprintf("/api/events/%s", url.PathEscape(eventType)), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(result)
}
