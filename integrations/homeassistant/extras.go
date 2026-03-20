package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func getConfig(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.get(ctx, "/api/config")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func checkConfig(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.post(ctx, "/api/config/core/check_config", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func renderTemplate(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tmpl := r.Str("template")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tmpl == "" {
		return mcp.ErrResult(fmt.Errorf("template is required"))
	}

	raw, err := h.doRequestRaw(ctx, "POST", "/api/template", map[string]string{"template": tmpl})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(raw)}, nil
}

func getErrorLog(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	raw, err := h.doRequestRaw(ctx, "GET", "/api/error_log", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: string(raw)}, nil
}

func listCalendars(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.get(ctx, "/api/calendars")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCalendarEvents(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	entityID := r.Str("entity_id")
	start := r.Str("start")
	end := r.Str("end")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if entityID == "" || start == "" || end == "" {
		return mcp.ErrResult(fmt.Errorf("entity_id, start, and end are required"))
	}

	qs := queryEncode(map[string]string{"start": start, "end": end})
	data, err := h.get(ctx, "/api/calendars/%s%s", url.PathEscape(entityID), qs)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func handleIntent(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}

	body := map[string]any{"name": name}
	if v := r.Str("data"); v != "" {
		var d map[string]any
		if err := json.Unmarshal([]byte(v), &d); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for data: %w", err))
		}
		body["data"] = d
	}

	data, err := h.post(ctx, "/api/intent/handle", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
