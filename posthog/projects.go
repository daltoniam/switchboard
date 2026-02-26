package posthog

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"limit":  argStr(args, "limit"),
		"offset": argStr(args, "offset"),
	})
	data, err := p.get(ctx, "/api/projects/%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.get(ctx, "/api/projects/%s/", p.proj(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{"name": argStr(args, "name")}
	data, err := p.post(ctx, "/api/projects/", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	path := fmt.Sprintf("/api/projects/%s/", p.proj(args))
	data, err := p.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteProject(ctx context.Context, p *posthog, args map[string]any) (*mcp.ToolResult, error) {
	data, err := p.del(ctx, "/api/projects/%s/", argStr(args, "project_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
