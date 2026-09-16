package gitlab

import (
	"context"
	"fmt"
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

func handleListIssues(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	q := paginationQuery(r)
	if state := r.Str("state"); state != "" {
		q.Set("state", state)
	} else {
		q.Set("state", "opened")
	}
	if labels := r.Str("labels"); labels != "" {
		q.Set("labels", labels)
	}
	if search := r.Str("search"); search != "" {
		q.Set("search", search)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/issues", projectID)
	raw, _, err := g.doJSON(ctx, http.MethodGet, path, q, nil)
	return jsonResult(raw, err)
}

func handleGetIssue(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	iid := r.Int("issue_iid")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/issues/%d", projectID, iid)
	raw, _, err := g.doJSON(ctx, http.MethodGet, path, nil, nil)
	return jsonResult(raw, err)
}
