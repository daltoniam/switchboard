package homeassistant

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getHistory(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	entityID := r.Str("entity_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if entityID == "" {
		return mcp.ErrResult(fmt.Errorf("entity_id is required"))
	}

	path := "/api/history/period"
	if v := r.Str("start_time"); v != "" {
		path += "/" + url.PathEscape(v)
	}

	params := url.Values{}
	params.Set("filter_entity_id", entityID)
	if v := r.Str("end_time"); v != "" {
		params.Set("end_time", v)
	}
	if r.Bool("minimal_response") {
		params.Set("minimal_response", "")
	}
	if r.Bool("no_attributes") {
		params.Set("no_attributes", "")
	}
	if r.Bool("significant_changes_only") {
		params.Set("significant_changes_only", "")
	}

	encoded := params.Encode()
	encoded = strings.ReplaceAll(encoded, "minimal_response=", "minimal_response")
	encoded = strings.ReplaceAll(encoded, "no_attributes=", "no_attributes")
	encoded = strings.ReplaceAll(encoded, "significant_changes_only=", "significant_changes_only")

	data, err := h.get(ctx, "%s?%s", path, encoded)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLogbook(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	path := "/api/logbook"
	if v := r.Str("start_time"); v != "" {
		path += "/" + url.PathEscape(v)
	}

	params := map[string]string{}
	if v := r.Str("entity_id"); v != "" {
		params["entity"] = v
	}
	if v := r.Str("end_time"); v != "" {
		params["end_time"] = v
	}

	data, err := h.get(ctx, "%s%s", path, queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
