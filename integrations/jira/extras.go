package jira

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// ── Metadata ─────────────────────────────────────────────────────────

func listIssueTypes(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/issuetype")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listPriorities(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/priority")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listStatuses(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/status")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listLabels(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/label")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listFields(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/field")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listFilters(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argStr(args, "filter_name"); v != "" {
		params["filterName"] = v
	}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/filter/search%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getFilter(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/filter/%s", argStr(args, "filter_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Worklogs ─────────────────────────────────────────────────────────

func listWorklogs(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	q := queryEncode(params)
	data, err := j.get(ctx, "/issue/%s/worklog%s", argStr(args, "issue_key"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func addWorklog(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"timeSpent": argStr(args, "time_spent"),
	}
	if v := argStr(args, "comment"); v != "" {
		body["comment"] = textToADF(v)
	}
	if v := argStr(args, "started"); v != "" {
		body["started"] = v
	}
	path := fmt.Sprintf("/issue/%s/worklog", argStr(args, "issue_key"))
	data, err := j.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Server Info ──────────────────────────────────────────────────────

func getServerInfo(ctx context.Context, j *jira, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := j.get(ctx, "/serverInfo")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
