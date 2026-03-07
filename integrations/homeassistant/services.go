package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listServices(ctx context.Context, h *homeassistant, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := h.get(ctx, "/api/services")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func callService(ctx context.Context, h *homeassistant, args map[string]any) (*mcp.ToolResult, error) {
	domain := argStr(args, "domain")
	service := argStr(args, "service")
	if domain == "" || service == "" {
		return errResult(fmt.Errorf("domain and service are required"))
	}

	var body any
	if v := argStr(args, "service_data"); v != "" {
		var data map[string]any
		if err := json.Unmarshal([]byte(v), &data); err != nil {
			return errResult(fmt.Errorf("invalid JSON for service_data: %w", err))
		}
		body = data
	}

	path := fmt.Sprintf("/api/services/%s/%s", url.PathEscape(domain), url.PathEscape(service))
	if argBool(args, "return_response") {
		path += "?return_response"
	}

	result, err := h.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(result)
}
