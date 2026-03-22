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
	r := mcp.NewArgs(args)
	id := r.Str("automation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("automation_id is required"))
	}
	data, err := h.get(ctx, "/api/config/automation/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func saveAutomation(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("automation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("automation_id is required"))
	}
	configJSON := r.Str("config")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if configJSON == "" {
		return mcp.ErrResult(fmt.Errorf("config is required"))
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for config: %w", err))
	}
	data, err := h.postf(ctx, config, "/api/config/automation/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteAutomation(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("automation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("automation_id is required"))
	}
	data, err := h.del(ctx, "/api/config/automation/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Scenes ---

func getScene(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("scene_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("scene_id is required"))
	}
	data, err := h.get(ctx, "/api/config/scene/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func saveScene(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("scene_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("scene_id is required"))
	}
	configJSON := r.Str("config")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if configJSON == "" {
		return mcp.ErrResult(fmt.Errorf("config is required"))
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for config: %w", err))
	}
	data, err := h.postf(ctx, config, "/api/config/scene/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteScene(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("scene_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("scene_id is required"))
	}
	data, err := h.del(ctx, "/api/config/scene/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Scripts ---

func getScript(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("script_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("script_id is required"))
	}
	data, err := h.get(ctx, "/api/config/script/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func saveScript(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("script_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("script_id is required"))
	}
	configJSON := r.Str("config")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if configJSON == "" {
		return mcp.ErrResult(fmt.Errorf("config is required"))
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return mcp.ErrResult(fmt.Errorf("invalid JSON for config: %w", err))
	}
	data, err := h.postf(ctx, config, "/api/config/script/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteScript(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("script_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("script_id is required"))
	}
	data, err := h.del(ctx, "/api/config/script/config/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
