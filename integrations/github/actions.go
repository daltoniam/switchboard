package github

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Workflows ─────────────────────────────────────────────────────

func listWorkflows(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	resp, _, err := g.client.Actions.ListWorkflows(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Workflows)
}

func listWorkflowRuns(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	branch := r.Str("branch")
	event := r.Str("event")
	status := r.Str("status")
	wfID := r.Str("workflow_id")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListWorkflowRunsOptions{
		Branch:      branch,
		Event:       event,
		Status:      status,
		ListOptions: gh.ListOptions{Page: page, PerPage: perPage},
	}

	var runs *gh.WorkflowRuns
	var err error
	if wfID != "" {
		runs, _, err = g.client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, wfID, opts)
	} else {
		runs, _, err = g.client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	}
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(runs.WorkflowRuns)
}

func getWorkflowRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	runID := r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	run, _, err := g.client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(run)
}

func listWorkflowJobs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	runID := r.Int64("run_id")
	filter := r.Str("filter")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListWorkflowJobsOptions{
		Filter:      filter,
		ListOptions: gh.ListOptions{Page: page, PerPage: perPage},
	}
	resp, _, err := g.client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Jobs)
}

func downloadWorkflowLogs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	runID := r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	url, _, err := g.client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 0)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"logs_url": url.String()})
}

func rerunWorkflow(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	runID := r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Actions.RerunWorkflowByID(ctx, owner, repo, runID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "rerun_requested"})
}

func cancelWorkflowRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	runID := r.Int64("run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Actions.CancelWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "cancelled"})
}

// ── Secrets ───────────────────────────────────────────────────────

func listRepoSecrets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	resp, _, err := g.client.Actions.ListRepoSecrets(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Secrets)
}

func listArtifacts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListArtifactsOptions{ListOptions: gh.ListOptions{Page: page, PerPage: perPage}}
	resp, _, err := g.client.Actions.ListArtifacts(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Artifacts)
}

func listEnvironmentSecrets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
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
	repoID, err := g.repoID(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	resp, _, err := g.client.Actions.ListEnvSecrets(ctx, repoID, environment, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Secrets)
}

func listOrgSecrets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	resp, _, err := g.client.Actions.ListOrgSecrets(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.Secrets)
}

// ── Checks ────────────────────────────────────────────────────────

func listCheckRuns(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{Page: page, PerPage: perPage}}
	resp, _, err := g.client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.CheckRuns)
}

func getCheckRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	checkRunID := r.Int64("check_run_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	run, _, err := g.client.Checks.GetCheckRun(ctx, owner, repo, checkRunID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(run)
}

func listCheckSuites(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListCheckSuiteOptions{ListOptions: gh.ListOptions{Page: page, PerPage: perPage}}
	resp, _, err := g.client.Checks.ListCheckSuitesForRef(ctx, owner, repo, ref, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(resp.CheckSuites)
}

// helper to get repo numeric ID for env secrets API
func (g *integration) repoID(ctx context.Context, owner, repo string) (int, error) {
	r, _, err := g.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return 0, fmt.Errorf("failed to get repo ID: %w", err)
	}
	return int(r.GetID()), nil
}
