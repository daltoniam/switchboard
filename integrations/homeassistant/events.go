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
		return errResult(err)
	}
	return rawResult(data)
}

func fireEvent(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	eventType := argStr(args, "event_type")
	if eventType == "" {
		return errResult(fmt.Errorf("event_type is required"))
	}

	var body any
	if v := argStr(args, "event_data"); v != "" {
		var data map[string]any
		if err := json.Unmarshal([]byte(v), &data); err != nil {
			return errResult(fmt.Errorf("invalid JSON for event_data: %w", err))
		}
		body = data
	}

	result, err := h.post(ctx, fmt.Sprintf("/api/events/%s", url.PathEscape(eventType)), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(result)
}
