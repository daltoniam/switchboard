package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// --- Automations ---

func getAutomation(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "automation_id")
	if id == "" {
		return errResult(fmt.Errorf("automation_id is required"))
	}
	data, err := h.get(ctx, "/api/config/automation/config/%s", url.PathEscape(id))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func saveAutomation(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "automation_id")
	if id == "" {
		return errResult(fmt.Errorf("automation_id is required"))
	}
	configJSON := argStr(args, "config")
	if configJSON == "" {
		return errResult(fmt.Errorf("config is required"))
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return errResult(fmt.Errorf("invalid JSON for config: %w", err))
	}
	data, err := h.post(ctx, fmt.Sprintf("/api/config/automation/config/%s", url.PathEscape(id)), config)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteAutomation(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "automation_id")
	if id == "" {
		return errResult(fmt.Errorf("automation_id is required"))
	}
	data, err := h.del(ctx, "/api/config/automation/config/%s", url.PathEscape(id))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Scenes ---

func getScene(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "scene_id")
	if id == "" {
		return errResult(fmt.Errorf("scene_id is required"))
	}
	data, err := h.get(ctx, "/api/config/scene/config/%s", url.PathEscape(id))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func saveScene(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "scene_id")
	if id == "" {
		return errResult(fmt.Errorf("scene_id is required"))
	}
	configJSON := argStr(args, "config")
	if configJSON == "" {
		return errResult(fmt.Errorf("config is required"))
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return errResult(fmt.Errorf("invalid JSON for config: %w", err))
	}
	data, err := h.post(ctx, fmt.Sprintf("/api/config/scene/config/%s", url.PathEscape(id)), config)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteScene(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "scene_id")
	if id == "" {
		return errResult(fmt.Errorf("scene_id is required"))
	}
	data, err := h.del(ctx, "/api/config/scene/config/%s", url.PathEscape(id))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Scripts ---

func getScript(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "script_id")
	if id == "" {
		return errResult(fmt.Errorf("script_id is required"))
	}
	data, err := h.get(ctx, "/api/config/script/config/%s", url.PathEscape(id))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func saveScript(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "script_id")
	if id == "" {
		return errResult(fmt.Errorf("script_id is required"))
	}
	configJSON := argStr(args, "config")
	if configJSON == "" {
		return errResult(fmt.Errorf("config is required"))
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return errResult(fmt.Errorf("invalid JSON for config: %w", err))
	}
	data, err := h.post(ctx, fmt.Sprintf("/api/config/script/config/%s", url.PathEscape(id)), config)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteScript(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "script_id")
	if id == "" {
		return errResult(fmt.Errorf("script_id is required"))
	}
	data, err := h.del(ctx, "/api/config/script/config/%s", url.PathEscape(id))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
