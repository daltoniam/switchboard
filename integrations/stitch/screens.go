package stitch

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listScreens(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if projectID == "" {
		return &mcp.ToolResult{Data: "project_id parameter is required", IsError: true}, nil
	}
	data, err := s.get(ctx, "/projects/%s/screens", url.PathEscape(projectID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getScreen(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return &mcp.ToolResult{Data: "name parameter is required", IsError: true}, nil
	}
	data, err := s.get(ctx, "/%s", name)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generateScreenFromText(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	prompt := r.Str("prompt")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if projectID == "" {
		return &mcp.ToolResult{Data: "project_id parameter is required", IsError: true}, nil
	}
	if prompt == "" {
		return &mcp.ToolResult{Data: "prompt parameter is required", IsError: true}, nil
	}

	body := map[string]any{
		"projectId": projectID,
		"prompt":    prompt,
	}
	deviceType, err := mcp.ArgStr(args, "device_type")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if deviceType != "" {
		body["deviceType"] = deviceType
	}
	modelID, err := mcp.ArgStr(args, "model_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if modelID != "" {
		body["modelId"] = modelID
	}

	path := fmt.Sprintf("/projects/%s/screens:generateFromText", url.PathEscape(projectID))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func editScreens(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	prompt := r.Str("prompt")
	screenIDs := r.StrSlice("selected_screen_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if projectID == "" {
		return &mcp.ToolResult{Data: "project_id parameter is required", IsError: true}, nil
	}
	if len(screenIDs) == 0 {
		return &mcp.ToolResult{Data: "selected_screen_ids parameter is required", IsError: true}, nil
	}
	if prompt == "" {
		return &mcp.ToolResult{Data: "prompt parameter is required", IsError: true}, nil
	}

	body := map[string]any{
		"projectId":         projectID,
		"selectedScreenIds": screenIDs,
		"prompt":            prompt,
	}
	deviceType, err := mcp.ArgStr(args, "device_type")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if deviceType != "" {
		body["deviceType"] = deviceType
	}
	modelID, err := mcp.ArgStr(args, "model_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if modelID != "" {
		body["modelId"] = modelID
	}

	path := fmt.Sprintf("/projects/%s/screens:edit", url.PathEscape(projectID))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func generateVariants(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := r.Str("project_id")
	prompt := r.Str("prompt")
	screenIDs := r.StrSlice("selected_screen_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if projectID == "" {
		return &mcp.ToolResult{Data: "project_id parameter is required", IsError: true}, nil
	}
	if len(screenIDs) == 0 {
		return &mcp.ToolResult{Data: "selected_screen_ids parameter is required", IsError: true}, nil
	}
	if prompt == "" {
		return &mcp.ToolResult{Data: "prompt parameter is required", IsError: true}, nil
	}

	variantOpts := map[string]any{}

	creativeRange, err := mcp.ArgStr(args, "creative_range")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if creativeRange != "" {
		variantOpts["creativeRange"] = creativeRange
	}

	variantCount, err := mcp.ArgInt(args, "variant_count")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if variantCount > 0 {
		variantOpts["variantCount"] = variantCount
	}

	aspects, err := mcp.ArgStrSlice(args, "aspects")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(aspects) > 0 {
		variantOpts["aspects"] = aspects
	}

	body := map[string]any{
		"projectId":         projectID,
		"selectedScreenIds": screenIDs,
		"prompt":            prompt,
		"variantOptions":    variantOpts,
	}

	deviceType, err := mcp.ArgStr(args, "device_type")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if deviceType != "" {
		body["deviceType"] = deviceType
	}
	modelID, err := mcp.ArgStr(args, "model_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if modelID != "" {
		body["modelId"] = modelID
	}

	path := fmt.Sprintf("/projects/%s/screens:generateVariants", url.PathEscape(projectID))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
