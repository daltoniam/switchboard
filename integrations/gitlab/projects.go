package gitlab

import (
	"context"
	"net/http"

	mcp "github.com/daltoniam/switchboard"
)

func handleGetCurrentUser(ctx context.Context, g *gitlab, _ map[string]any) (*mcp.ToolResult, error) {
	raw, _, err := g.doJSON(ctx, http.MethodGet, "/user", nil, nil)
	return jsonResult(raw, err)
}

func handleListProjects(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	q := paginationQuery(r)
	q.Set("membership", "true")
	if args != nil {
		if v, ok := args["membership"]; ok {
			if b, ok := v.(bool); ok && !b {
				q.Del("membership")
			}
		}
	}
	if s := r.Str("search"); s != "" {
		q.Set("search", s)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	raw, _, err := g.doJSON(ctx, http.MethodGet, "/projects", q, nil)
	return jsonResult(raw, err)
}

func handleGetProject(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	raw, _, err := g.doJSON(ctx, http.MethodGet, "/projects/"+projectID, nil, nil)
	return jsonResult(raw, err)
}
