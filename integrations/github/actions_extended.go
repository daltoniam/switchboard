package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Workflows Extended ────────────────────────────────────────────

func triggerWorkflow(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	workflowID := r.Str("workflow_id")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	event := gh.CreateWorkflowDispatchEventRequest{
		Ref: ref,
	}
	_, err := g.client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, workflowID, event)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "dispatched"})
}

func rerunFailedJobs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	runID := r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Actions.RerunFailedJobsByID(ctx, owner, repo, runID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "rerun_failed_requested"})
}

func getWorkflowJob(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	jobID := r.Int64("job_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	job, _, err := g.client.Actions.GetWorkflowJobByID(ctx, owner, repo, jobID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(job)
}

func getWorkflowJobLogs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	jobID := r.Int64("job_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	url, _, err := g.client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 0)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"logs_url": url.String()})
}

func deleteWorkflowRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	runID := r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Actions.DeleteWorkflowRun(ctx, owner, repo, runID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

// ── Actions Variables ─────────────────────────────────────────────

func listRepoVariables(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	resp, _, err := g.client.Actions.ListRepoVariables(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Variables)
}

func createRepoVariable(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	name := r.Str("name")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	v := &gh.ActionsVariable{
		Name:  name,
		Value: value,
	}
	_, err := g.client.Actions.CreateRepoVariable(ctx, owner, repo, v)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "created", "name": v.Name})
}

func updateRepoVariable(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	name := r.Str("name")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	v := &gh.ActionsVariable{
		Name:  name,
		Value: value,
	}
	_, err := g.client.Actions.UpdateRepoVariable(ctx, owner, repo, v)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "updated", "name": v.Name})
}

func deleteRepoVariable(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Actions.DeleteRepoVariable(ctx, owner, repo, name)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

func listOrgVariables(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	resp, _, err := g.client.Actions.ListOrgVariables(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Variables)
}

func listEnvVariables(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	environment := r.Str("environment")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	resp, _, err := g.client.Actions.ListEnvVariables(ctx, owner, repo, environment, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Variables)
}

// ── Runners ───────────────────────────────────────────────────────

func listRunners(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListRunnersOptions{ListOptions: gh.ListOptions{Page: page, PerPage: perPage}}
	resp, _, err := g.client.Actions.ListRunners(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Runners)
}

func listOrgRunners(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListRunnersOptions{ListOptions: gh.ListOptions{Page: page, PerPage: perPage}}
	resp, _, err := g.client.Actions.ListOrganizationRunners(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Runners)
}
