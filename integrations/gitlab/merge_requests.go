package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func handleListMergeRequests(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	q := paginationQuery(r)
	if state := r.Str("state"); state != "" {
		q.Set("state", state)
	} else {
		q.Set("state", "opened")
	}
	if scope := r.Str("scope"); scope != "" {
		q.Set("scope", scope)
	}
	if search := r.Str("search"); search != "" {
		q.Set("search", search)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/merge_requests", projectID)
	raw, _, err := g.doJSON(ctx, http.MethodGet, path, q, nil)
	return jsonResult(raw, err)
}

func handleGetMergeRequest(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	iid := r.Int("merge_request_iid")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", projectID, iid)
	raw, _, err := g.doJSON(ctx, http.MethodGet, path, nil, nil)
	return jsonResult(raw, err)
}

func handleCreateMergeRequestNote(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	iid := r.Int("merge_request_iid")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/notes", projectID, iid)
	raw, _, err := g.doJSON(ctx, http.MethodPost, path, url.Values{}, map[string]string{"body": body})
	return jsonResult(raw, err)
}
