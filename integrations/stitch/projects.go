package stitch

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{"filter": filter}
	data, err := s.get(ctx, "/projects%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProject(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if title != "" {
		body["title"] = title
	}
	data, err := s.post(ctx, "/projects", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getProject(ctx context.Context, s *stitch, args map[string]any) (*mcp.ToolResult, error) {
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
