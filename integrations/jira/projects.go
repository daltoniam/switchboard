package jira

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "query"); v != "" {
		params["query"] = v
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/project/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getProject(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/project/%s", argStr(args, "project_key"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectComponents(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/project/%s/components", argStr(args, "project_key"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectVersions(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/project/%s/versions", argStr(args, "project_key"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectStatuses(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/project/%s/statuses", argStr(args, "project_key"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
