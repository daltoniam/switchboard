package homeassistant

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getHistory(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	entityID := argStr(args, "entity_id")
	if entityID == "" {
		return errResult(fmt.Errorf("entity_id is required"))
	}

	path := "/api/history/period"
	if v := argStr(args, "start_time"); v != "" {
		path += "/" + url.PathEscape(v)
	}

	params := url.Values{}
	params.Set("filter_entity_id", entityID)
	if v := argStr(args, "end_time"); v != "" {
		params.Set("end_time", v)
	}
	if argBool(args, "minimal_response") {
		params.Set("minimal_response", "")
	}
	if argBool(args, "no_attributes") {
		params.Set("no_attributes", "")
	}
	if argBool(args, "significant_changes_only") {
		params.Set("significant_changes_only", "")
	}

	encoded := params.Encode()
	encoded = strings.ReplaceAll(encoded, "minimal_response=", "minimal_response")
	encoded = strings.ReplaceAll(encoded, "no_attributes=", "no_attributes")
	encoded = strings.ReplaceAll(encoded, "significant_changes_only=", "significant_changes_only")

	data, err := h.get(ctx, "%s?%s", path, encoded)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getLogbook(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	path := "/api/logbook"
	if v := argStr(args, "start_time"); v != "" {
		path += "/" + url.PathEscape(v)
	}

	params := map[string]string{}
	if v := argStr(args, "entity_id"); v != "" {
		params["entity"] = v
	}
	if v := argStr(args, "end_time"); v != "" {
		params["end_time"] = v
	}

	data, err := h.get(ctx, "%s%s", path, queryEncode(params))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
