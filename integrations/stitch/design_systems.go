package stitch

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listDesignSystems(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if projectID != "" {
		data, err := s.get(ctx, "/projects/%s/designSystems", url.PathEscape(projectID))
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}
	data, err := s.get(ctx, "/designSystems")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDesignSystem(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	displayName := r.Str("display_name")
	colorMode := r.Str("color_mode")
	headlineFont := r.Str("headline_font")
	bodyFont := r.Str("body_font")
	roundness := r.Str("roundness")
	customColor := r.Str("custom_color")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if displayName == "" {
		return &mcp.ToolResult{Data: "display_name parameter is required", IsError: true}, nil
	}

	theme := map[string]any{
		"colorMode":    colorMode,
		"headlineFont": headlineFont,
		"bodyFont":     bodyFont,
		"roundness":    roundness,
		"customColor":  customColor,
	}

	colorVariant, err := mcp.ArgStr(args, "color_variant")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if colorVariant != "" {
		theme["colorVariant"] = colorVariant
	}

	designMd, err := mcp.ArgStr(args, "design_md")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if designMd != "" {
		theme["designMd"] = designMd
	}

	body := map[string]any{
		"designSystem": map[string]any{
			"displayName": displayName,
			"theme":       theme,
		},
	}

	projectID, err := mcp.ArgStr(args, "project_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if projectID != "" {
		body["projectId"] = projectID
	}

	data, err := s.post(ctx, "/designSystems", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDesignSystem(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	projectID := r.Str("project_id")
	displayName := r.Str("display_name")
	colorMode := r.Str("color_mode")
	headlineFont := r.Str("headline_font")
	bodyFont := r.Str("body_font")
	roundness := r.Str("roundness")
	customColor := r.Str("custom_color")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	if name == "" {
		return &mcp.ToolResult{Data: "name parameter is required", IsError: true}, nil
	}

	theme := map[string]any{
		"colorMode":    colorMode,
		"headlineFont": headlineFont,
		"bodyFont":     bodyFont,
		"roundness":    roundness,
		"customColor":  customColor,
	}

	colorVariant, err := mcp.ArgStr(args, "color_variant")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if colorVariant != "" {
		theme["colorVariant"] = colorVariant
	}

	designMd, err := mcp.ArgStr(args, "design_md")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if designMd != "" {
		theme["designMd"] = designMd
	}

	body := map[string]any{
		"name":      name,
		"projectId": projectID,
		"designSystem": map[string]any{
			"displayName": displayName,
			"theme":       theme,
		},
	}

	path := fmt.Sprintf("/%s", name)
	data, err := s.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func applyDesignSystem(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	assetID := r.Str("asset_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if projectID == "" {
		return &mcp.ToolResult{Data: "project_id parameter is required", IsError: true}, nil
	}
	if assetID == "" {
		return &mcp.ToolResult{Data: "asset_id parameter is required", IsError: true}, nil
	}

	// selected_screen_instances is an array of objects, pass through as-is.
	rawInstances, ok := args["selected_screen_instances"]
	if !ok || rawInstances == nil {
		return &mcp.ToolResult{Data: "selected_screen_instances parameter is required", IsError: true}, nil
	}

	// Convert snake_case keys to camelCase for each instance.
	instances, err := convertScreenInstances(rawInstances)
	if err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"projectId":               projectID,
		"assetId":                 assetID,
		"selectedScreenInstances": instances,
	}

	path := fmt.Sprintf("/projects/%s/screens:applyDesignSystem", url.PathEscape(projectID))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// convertScreenInstances normalizes screen instance input, accepting both
// snake_case (source_screen) and camelCase (sourceScreen) keys.
func convertScreenInstances(raw any) ([]map[string]any, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("selected_screen_instances must be an array")
	}
	out := make([]map[string]any, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("selected_screen_instances[%d] must be an object", i)
		}
		inst := map[string]any{}
		if v, ok := m["id"]; ok {
			inst["id"] = v
		}
		if v, ok := m["source_screen"]; ok {
			inst["sourceScreen"] = v
		} else if v, ok := m["sourceScreen"]; ok {
			inst["sourceScreen"] = v
		}
		out = append(out, inst)
	}
	return out, nil
}
