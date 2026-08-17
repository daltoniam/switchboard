package jira

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("query"); v != "" {
		params["query"] = v
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/project/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getProject(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/project/%s", url.PathEscape(projectKey))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectComponents(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/project/%s/components", url.PathEscape(projectKey))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectVersions(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/project/%s/versions", url.PathEscape(projectKey))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectStatuses(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/project/%s/statuses", url.PathEscape(projectKey))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
