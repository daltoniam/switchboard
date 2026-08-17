package jira

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// ── Metadata ─────────────────────────────────────────────────────────

func listIssueTypes(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/issuetype")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPriorities(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/priority")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listStatuses(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/status")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listLabels(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/label")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listFields(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/field")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listFilters(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Str("filter_name"); v != "" {
		params["filterName"] = v
	}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/filter/search%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFilter(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filterID := r.Str("filter_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := j.get(ctx, "/filter/%s", url.PathEscape(filterID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Worklogs ─────────────────────────────────────────────────────────

func listWorklogs(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/issue/%s/worklog%s", url.PathEscape(issueKey), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addWorklog(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"timeSpent": r.Str("time_spent"),
	}
	if v := r.Str("comment"); v != "" {
		body["comment"] = textToADF(v)
	}
	if v := r.Str("started"); v != "" {
		body["started"] = v
	}
	issueKey := r.Str("issue_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/issue/%s/worklog", url.PathEscape(issueKey))
	data, err := j.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Server Info ──────────────────────────────────────────────────────

func getServerInfo(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/serverInfo")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
