package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	mcp "github.com/daltoniam/switchboard"
)

var actionRunStatuses = []string{"unknown", "waiting", "running", "success", "failure", "cancelled", "skipped", "blocked"}

type actionRunJob struct {
	ID      int64    `json:"id"`
	RunID   int64    `json:"run_id"`
	Attempt int64    `json:"attempt"`
	Handle  string   `json:"handle"`
	RepoID  int64    `json:"repo_id"`
	OwnerID int64    `json:"owner_id"`
	Name    string   `json:"name"`
	Needs   []string `json:"needs"`
	RunsOn  []string `json:"runs_on"`
	TaskID  int64    `json:"task_id"`
	Status  string   `json:"status"`
}

func listActionRuns(ctx context.Context, f *forgejo, _ *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo := r.Str("owner"), r.Str("repo")
	opts := pagination(r)
	event, status, runNumber, headSHA := r.Str("event"), r.Str("status"), r.Int64("run_number"), r.Str("head_sha")
	workflowID, ref := r.Str("workflow_id"), r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	query := url.Values{
		"page":  {strconv.Itoa(opts.Page)},
		"limit": {strconv.Itoa(opts.PageSize)},
	}
	if event != "" {
		query.Set("event", event)
	}
	if status != "" {
		query.Set("status", status)
	}
	if runNumber > 0 {
		query.Set("run_number", strconv.FormatInt(runNumber, 10))
	}
	if headSHA != "" {
		query.Set("head_sha", headSHA)
	}
	if workflowID != "" {
		query.Set("workflow_id", workflowID)
	}
	if ref != "" {
		query.Set("ref", ref)
	}
	var runs sdk.ListActionRunsResponse
	resp, err := f.apiJSON(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/actions/runs?%s", owner, repo, query.Encode()), &runs)
	if err != nil || (resp != nil && resp.Response != nil && resp.StatusCode >= 300) {
		return finish(nil, resp, err)
	}
	filtered := runs.WorkflowRuns
	if filtered == nil {
		filtered = []*sdk.ActionRun{}
	}
	return finish(filtered, resp, nil)
}

func getActionRun(c *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, runID := r.Str("owner"), r.Str("repo"), r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	return finish(c.GetRepoActionRun(owner, repo, runID))
}

func listActionJobs(ctx context.Context, f *forgejo, _ *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, runID := r.Str("owner"), r.Str("repo"), r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	jobs := make([]*actionRunJob, 0)
	resp, err := f.apiJSON(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, repo, runID), &jobs)
	if jobs == nil {
		jobs = []*actionRunJob{}
	}
	return finish(jobs, resp, err)
}

func getActionJobLogs(ctx context.Context, f *forgejo, _ *sdk.Client, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner, repo, jobID, attempt := r.Str("owner"), r.Str("repo"), r.Int64("job_id"), r.Int64("attempt")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", owner, repo, jobID)
	if _, ok := args["attempt"]; ok {
		path += "?" + url.Values{"attempt": {strconv.FormatInt(attempt, 10)}}.Encode()
	}
	data, resp, err := f.apiBytes(ctx, http.MethodGet, path, "text/plain")
	return finish(map[string]string{"logs": string(data)}, resp, err)
}

func (f *forgejo) apiJSON(ctx context.Context, method, path string, dest any) (*sdk.Response, error) {
	data, resp, err := f.apiBytes(ctx, method, path, "application/json")
	if err != nil || dest == nil {
		return resp, err
	}
	if len(data) == 0 {
		return resp, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return resp, err
	}
	return resp, nil
}

func (f *forgejo) apiBytes(ctx context.Context, method, path, accept string) ([]byte, *sdk.Response, error) {
	if f == nil {
		return nil, nil, mcp.ErrNotConfigured
	}
	f.mu.RLock()
	baseURL, token, hc := f.baseURL, f.token, f.httpClient
	f.mu.RUnlock()
	if hc == nil || baseURL == "" || token == "" {
		return nil, nil, mcp.ErrNotConfigured
	}
	if ctx == nil {
		return nil, nil, fmt.Errorf("forgejo: context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+"/api/v1"+path, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", accept)
	httpResp, err := hc.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()
	resp := &sdk.Response{Response: httpResp}
	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, resp, err
	}
	if httpResp.StatusCode/100 != 2 {
		message := string(data)
		var payload map[string]any
		if json.Unmarshal(data, &payload) == nil {
			if msg, ok := payload["message"]; ok {
				message = fmt.Sprint(msg)
			}
		}
		if message == "" {
			return data, resp, fmt.Errorf("forgejo: HTTP %d %s", httpResp.StatusCode, http.StatusText(httpResp.StatusCode))
		}
		return data, resp, fmt.Errorf("%s", message)
	}
	return data, resp, nil
}
