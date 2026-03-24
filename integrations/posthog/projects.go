package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := queryEncode(map[string]string{
		"limit":  r.Str("limit"),
		"offset": r.Str("offset"),
	})
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.get(ctx, "/api/projects/%s/", projID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	name, err := mcp.ArgStr(args, "name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"name": name}
	data, err := p.post(ctx, "/api/projects/", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projID, err := p.proj(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	name, err := mcp.ArgStr(args, "name")
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	path := fmt.Sprintf("/api/projects/%s/", projID)
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	projectID, err := mcp.ArgStr(args, "project_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := p.del(ctx, "/api/projects/%s/", projectID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
