package gong

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func parseJSONArray(raw string) ([]any, error) {
	if raw == "" {
		return nil, nil
	}
	var out []any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON array: %w", err)
	}
	return out, nil
}

func listCalls(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	from := r.Str("from_date_time")
	to := r.Str("to_date_time")
	workspaceID := r.Str("workspace_id")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"fromDateTime": from,
		"toDateTime":   to,
		"workspaceId":  workspaceID,
		"cursor":       cursor,
	}
	data, err := g.get(ctx, "/v2/calls%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCall(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("call_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/v2/calls/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCallsExtensive(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	from := r.Str("from_date_time")
	to := r.Str("to_date_time")
	callIDsRaw := r.Str("call_ids")
	workspaceID := r.Str("workspace_id")
	cursor := r.Str("cursor")
	contentSelectorRaw := r.Str("content_selector")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	filter := map[string]any{}
	if from != "" {
		filter["fromDateTime"] = from
	}
	if to != "" {
		filter["toDateTime"] = to
	}
	if workspaceID != "" {
		filter["workspaceId"] = workspaceID
	}
	if callIDsRaw != "" {
		ids, err := parseJSONArray(callIDsRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		filter["callIds"] = ids
	}
	body := map[string]any{"filter": filter}
	if cursor != "" {
		body["cursor"] = cursor
	}
	if contentSelectorRaw != "" {
		var cs any
		if err := json.Unmarshal([]byte(contentSelectorRaw), &cs); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid JSON for content_selector: %w", err))
		}
		body["contentSelector"] = cs
	}
	data, err := g.post(ctx, "/v2/calls/extensive", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTranscripts(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	from := r.Str("from_date_time")
	to := r.Str("to_date_time")
	callIDsRaw := r.Str("call_ids")
	workspaceID := r.Str("workspace_id")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	filter := map[string]any{}
	if from != "" {
		filter["fromDateTime"] = from
	}
	if to != "" {
		filter["toDateTime"] = to
	}
	if workspaceID != "" {
		filter["workspaceId"] = workspaceID
	}
	if callIDsRaw != "" {
		ids, err := parseJSONArray(callIDsRaw)
		if err != nil {
			return mcp.ErrResult(err)
		}
		filter["callIds"] = ids
	}
	body := map[string]any{"filter": filter}
	if cursor != "" {
		body["cursor"] = cursor
	}
	data, err := g.post(ctx, "/v2/calls/transcript", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUsers(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/v2/users%s", queryEncode(cursorParam(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUser(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/v2/users/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUsersExtensive(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userIDsRaw := r.Str("user_ids")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ids, err := parseJSONArray(userIDsRaw)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"filter": map[string]any{"userIds": ids},
	}
	if cursor != "" {
		body["cursor"] = cursor
	}
	data, err := g.post(ctx, "/v2/users/extensive", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listWorkspaces(ctx context.Context, g *gong, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/v2/workspaces")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listLibraryFolders(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	workspaceID := r.Str("workspace_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{"workspaceId": workspaceID}
	data, err := g.get(ctx, "/v2/library/folders%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLibraryFolder(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	folderID := r.Str("folder_id")
	workspaceID := r.Str("workspace_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"folderId":    folderID,
		"workspaceId": workspaceID,
	}
	data, err := g.get(ctx, "/v2/library/folder-content%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func statsBody(args map[string]any) (map[string]any, error) {
	r := mcp.NewArgs(args)
	from := r.Str("from_date")
	to := r.Str("to_date")
	workspaceID := r.Str("workspace_id")
	userIDsRaw := r.Str("user_ids")
	if err := r.Err(); err != nil {
		return nil, err
	}
	body := map[string]any{
		"filter": map[string]any{
			"fromDate": from,
			"toDate":   to,
		},
	}
	filter := body["filter"].(map[string]any)
	if workspaceID != "" {
		filter["workspaceId"] = workspaceID
	}
	if userIDsRaw != "" {
		ids, err := parseJSONArray(userIDsRaw)
		if err != nil {
			return nil, err
		}
		filter["userIds"] = ids
	}
	return body, nil
}

func listStatsActivity(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	body, err := statsBody(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, "/v2/stats/activity/aggregate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listStatsInteraction(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	body, err := statsBody(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, "/v2/stats/interaction", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listStatsScorecards(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	body, err := statsBody(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.post(ctx, "/v2/stats/activity/scorecards", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listLogs(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	from := r.Str("from_date_time")
	to := r.Str("to_date_time")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"fromDateTime": from,
		"toDateTime":   to,
		"cursor":       cursor,
	}
	data, err := g.get(ctx, "/v2/logs%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listDataPrivacy(ctx context.Context, g *gong, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/v2/data-privacy/data-for-all-users%s", queryEncode(cursorParam(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
