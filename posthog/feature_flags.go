package posthog

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listFeatureFlags(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"search": argStr(args, "search"),
		"active": argStr(args, "active"),
		"type":   argStr(args, "type"),
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/feature_flags/%s", p.proj(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/feature_flags/%s/", p.proj(args), argStr(args, "flag_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"key": argStr(args, "key")}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "filters"); v != "" {
		var filters any
		if err := json.Unmarshal([]byte(v), &filters); err == nil {
			body["filters"] = filters
		}
	}
	if _, ok := args["active"]; ok {
		body["active"] = argBool(args, "active")
	}
	if _, ok := args["ensure_experience_continuity"]; ok {
		body["ensure_experience_continuity"] = argBool(args, "ensure_experience_continuity")
	}
	path := fmt.Sprintf("/api/projects/%s/feature_flags/", p.proj(args))
	data, err := p.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "key"); v != "" {
		body["key"] = v
	}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "filters"); v != "" {
		var filters any
		if err := json.Unmarshal([]byte(v), &filters); err == nil {
			body["filters"] = filters
		}
	}
	if _, ok := args["active"]; ok {
		body["active"] = argBool(args, "active")
	}
	path := fmt.Sprintf("/api/projects/%s/feature_flags/%s/", p.proj(args), argStr(args, "flag_id"))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/api/projects/%s/feature_flags/%s/", p.proj(args), argStr(args, "flag_id"))
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func featureFlagActivity(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s/feature_flags/%s/activity/%s", p.proj(args), argStr(args, "flag_id"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
