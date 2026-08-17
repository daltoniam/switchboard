package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listFeatureFlags(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"search": r.Str("search"),
		"active": r.Str("active"),
		"type":   r.Str("type"),
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/feature_flags/%s", projID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	flagID, err := mcp.ArgStr(args, "flag_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/feature_flags/%s/", projID, flagID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	key := r.Str("key")
	name := r.Str("name")
	active := r.Bool("active")
	ensureExpCont := r.Bool("ensure_experience_continuity")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"key": key}
	if name != "" {
		body["name"] = name
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	if _, ok := args["active"]; ok {
		body["active"] = active
	}
	if _, ok := args["ensure_experience_continuity"]; ok {
		body["ensure_experience_continuity"] = ensureExpCont
	}
	path := fmt.Sprintf("/api/projects/%s/feature_flags/", projID)
	data, err := p.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	flagID := r.Str("flag_id")
	key := r.Str("key")
	name := r.Str("name")
	active := r.Bool("active")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if key != "" {
		body["key"] = key
	}
	if name != "" {
		body["name"] = name
	}
	if filters, err := parseJSON(args, "filters"); err != nil {
		return mcp.ErrResult(err)
	} else if filters != nil {
		body["filters"] = filters
	}
	if _, ok := args["active"]; ok {
		body["active"] = active
	}
	path := fmt.Sprintf("/api/projects/%s/feature_flags/%s/", projID, flagID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteFeatureFlag(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	flagID, err := mcp.ArgStr(args, "flag_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/api/projects/%s/feature_flags/%s/", projID, flagID)
	body := map[string]any{"deleted": true}
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func featureFlagActivity(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	flagID := r.Str("flag_id")
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/feature_flags/%s/activity/%s", projID, flagID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
