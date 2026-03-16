package jira

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listBoards(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "project_key"); v != "" {
		params["projectKeyOrId"] = v
	}
	if v := argStr(args, "type"); v != "" {
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
	data, err := j.agileGet(ctx, "/board/%s", argStr(args, "board_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listSprints(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "state"); v != "" {
		params["state"] = v
	}
	q := queryEncode(params)
	data, err := j.agileGet(ctx, "/board/%s/sprint%s", argStr(args, "board_id"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.agileGet(ctx, "/sprint/%s", argStr(args, "sprint_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"name":          argStr(args, "name"),
		"originBoardId": argInt(args, "board_id"),
	}
	if v := argStr(args, "start_date"); v != "" {
		body["startDate"] = v
	}
	if v := argStr(args, "end_date"); v != "" {
		body["endDate"] = v
	}
	if v := argStr(args, "goal"); v != "" {
		body["goal"] = v
	}
	data, err := j.agilePost(ctx, "/sprint", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "state"); v != "" {
		body["state"] = v
	}
	if v := argStr(args, "start_date"); v != "" {
		body["startDate"] = v
	}
	if v := argStr(args, "end_date"); v != "" {
		body["endDate"] = v
	}
	if v := argStr(args, "goal"); v != "" {
		body["goal"] = v
	}
	path := fmt.Sprintf("/sprint/%s", argStr(args, "sprint_id"))
	data, err := j.agilePut(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSprintIssues(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "jql"); v != "" {
		params["jql"] = v
	}
	q := queryEncode(params)
	data, err := j.agileGet(ctx, "/sprint/%s/issue%s", argStr(args, "sprint_id"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func moveIssuesToSprint(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	issuesStr := argStr(args, "issues")
	if issuesStr == "" {
		return mcp.ErrResult(fmt.Errorf("issues is required"))
	}
	issues := strings.Split(issuesStr, ",")
	body := map[string]any{"issues": issues}
	path := fmt.Sprintf("/sprint/%s/issue", argStr(args, "sprint_id"))
	data, err := j.agilePost(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBoardBacklog(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{}
	if v := argInt(args, "start_at"); v > 0 {
		params["startAt"] = fmt.Sprintf("%d", v)
	}
	if v := argInt(args, "max_results"); v > 0 {
		params["maxResults"] = fmt.Sprintf("%d", v)
	}
	if v := argStr(args, "jql"); v != "" {
		params["jql"] = v
	}
	q := queryEncode(params)
	data, err := j.agileGet(ctx, "/board/%s/backlog%s", argStr(args, "board_id"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBoardConfig(ctx context.Context, j *jira, args map[string]any) (*mcp.ToolResult, error) {
	data, err := j.agileGet(ctx, "/board/%s/configuration", argStr(args, "board_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
