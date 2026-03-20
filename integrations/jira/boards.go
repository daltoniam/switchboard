package jira

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listBoards(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("project_key"); v != "" {
		params["projectKeyOrId"] = v
	}
	if v := r.Str("type"); v != "" {
		params["type"] = v
	}
	q := queryEncode(params)
	data, err := j.agileGet(ctx, "/board%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBoard(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	data, err := j.agileGet(ctx, "/board/%s", url.PathEscape(r.Str("board_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listSprints(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("state"); v != "" {
		params["state"] = v
	}
	q := queryEncode(params)
	data, err := j.agileGet(ctx, "/board/%s/sprint%s", url.PathEscape(r.Str("board_id")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	data, err := j.agileGet(ctx, "/sprint/%s", url.PathEscape(r.Str("sprint_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"name":          r.Str("name"),
		"originBoardId": r.Int("board_id"),
	}
	if v := r.Str("start_date"); v != "" {
		body["startDate"] = v
	}
	if v := r.Str("end_date"); v != "" {
		body["endDate"] = v
	}
	if v := r.Str("goal"); v != "" {
		body["goal"] = v
	}
	data, err := j.agilePost(ctx, "/sprint", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{}
	if v := r.Str("name"); v != "" {
		body["name"] = v
	}
	if v := r.Str("state"); v != "" {
		body["state"] = v
	}
	if v := r.Str("start_date"); v != "" {
		body["startDate"] = v
	}
	if v := r.Str("end_date"); v != "" {
		body["endDate"] = v
	}
	if v := r.Str("goal"); v != "" {
		body["goal"] = v
	}
	path := fmt.Sprintf("/sprint/%s", url.PathEscape(r.Str("sprint_id")))
	data, err := j.agilePut(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSprintIssues(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("jql"); v != "" {
		params["jql"] = v
	}
	q := queryEncode(params)
	data, err := j.agileGet(ctx, "/sprint/%s/issue%s", url.PathEscape(r.Str("sprint_id")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func moveIssuesToSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issuesStr := r.Str("issues")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if issuesStr == "" {
		return mcp.ErrResult(fmt.Errorf("issues is required"))
	}
	issues := strings.Split(issuesStr, ",")
	for i, s := range issues {
		issues[i] = strings.TrimSpace(s)
	}
	body := map[string]any{"issues": issues}
	path := fmt.Sprintf("/sprint/%s/issue", url.PathEscape(r.Str("sprint_id")))
	data, err := j.agilePost(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBoardBacklog(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{}
	if v := r.Int("start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := r.Int("max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := r.Str("jql"); v != "" {
		params["jql"] = v
	}
	q := queryEncode(params)
	data, err := j.agileGet(ctx, "/board/%s/backlog%s", url.PathEscape(r.Str("board_id")), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBoardConfig(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	data, err := j.agileGet(ctx, "/board/%s/configuration", url.PathEscape(r.Str("board_id")))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
