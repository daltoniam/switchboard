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
		return errResult(err)
	}
	return rawResult(data)
}

func checkConfig(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.post(ctx, "/api/config/core/check_config", nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func renderTemplate(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	tmpl := argStr(args, "template")
	if tmpl == "" {
		return errResult(fmt.Errorf("template is required"))
	}

	raw, err := h.doRequestRaw(ctx, "POST", "/api/template", map[string]string{"template": tmpl})
	if err != nil {
		return errResult(err)
	}
	return &mcp.ToolResult{Data: string(raw)}, nil
}

func getErrorLog(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	raw, err := h.doRequestRaw(ctx, "GET", "/api/error_log", nil)
	if err != nil {
		return errResult(err)
	}
	return &mcp.ToolResult{Data: string(raw)}, nil
}

func listCalendars(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.get(ctx, "/api/calendars")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getCalendarEvents(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	entityID := argStr(args, "entity_id")
	start := argStr(args, "start")
	end := argStr(args, "end")
	if entityID == "" || start == "" || end == "" {
		return errResult(fmt.Errorf("entity_id, start, and end are required"))
	}

	qs := queryEncode(map[string]string{"start": start, "end": end})
	data, err := h.get(ctx, "/api/calendars/%s%s", url.PathEscape(entityID), qs)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func handleIntent(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "name")
	if name == "" {
		return errResult(fmt.Errorf("name is required"))
	}

	body := map[string]any{"name": name}
	if v := argStr(args, "data"); v != "" {
		var d map[string]any
		if err := json.Unmarshal([]byte(v), &d); err != nil {
			return errResult(fmt.Errorf("invalid JSON for data: %w", err))
		}
		body["data"] = d
	}

	data, err := h.post(ctx, "/api/intent/handle", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
