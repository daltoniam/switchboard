package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// MaxResponseBytesForTool raises the cap for gitlab_get_job_trace, which returns
// raw log text (not JSON) and needs headroom comparable to github_get_pull_diff.
func (g *gitlab) MaxResponseBytesForTool(name mcp.ToolName) (int, bool) {
	if name == "gitlab_get_job_trace" {
		return 1024 * 1024, true
	}
	return 0, false
}

func handleListPipelines(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	q := paginationQuery(r)
	if ref := r.Str("ref"); ref != "" {
		q.Set("ref", ref)
	}
	if status := r.Str("status"); status != "" {
		q.Set("status", status)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/pipelines", projectID)
	raw, _, err := g.doJSON(ctx, http.MethodGet, path, q, nil)
	return jsonResult(raw, err)
}

func handleListPipelineJobs(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	pipelineID := r.Int("pipeline_id")
	q := paginationQuery(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/pipelines/%d/jobs", projectID, pipelineID)
	raw, _, err := g.doJSON(ctx, http.MethodGet, path, q, nil)
	return jsonResult(raw, err)
}

func handleGetJobTrace(ctx context.Context, g *gitlab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectID := encodeProjectID(r.Str("project_id"))
	jobID := r.Int("job_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/projects/%s/jobs/%d/trace", projectID, jobID)
	trace, _, err := g.doText(ctx, path, url.Values{}, 1024*1024)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return &mcp.ToolResult{Data: trace}, nil
}
