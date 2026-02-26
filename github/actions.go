package github

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Workflows ─────────────────────────────────────────────────────

func listWorkflows(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	workflows, _, err := g.client.Actions.ListWorkflows(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(workflows)
}

func listWorkflowRuns(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListWorkflowRunsOptions{
		Branch:      argStr(args, "branch"),
		Event:       argStr(args, "event"),
		Status:      argStr(args, "status"),
		ListOptions: listOpts(args),
	}

	var runs *gh.WorkflowRuns
	var err error
	if wfID := argStr(args, "workflow_id"); wfID != "" {
		runs, _, err = g.client.Actions.ListWorkflowRunsByFileName(ctx, argStr(args, "owner"), argStr(args, "repo"), wfID, opts)
	} else {
		runs, _, err = g.client.Actions.ListRepositoryWorkflowRuns(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	}
	if err != nil {
		return errResult(err)
	}
	return jsonResult(runs)
}

func getWorkflowRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	run, _, err := g.client.Actions.GetWorkflowRunByID(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "run_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(run)
}

func listWorkflowJobs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListWorkflowJobsOptions{
		Filter:      argStr(args, "filter"),
		ListOptions: listOpts(args),
	}
	jobs, _, err := g.client.Actions.ListWorkflowJobs(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "run_id"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(jobs)
}

func downloadWorkflowLogs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	url, _, err := g.client.Actions.GetWorkflowRunLogs(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "run_id"), 0)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"logs_url": url.String()})
}

func rerunWorkflow(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Actions.RerunWorkflowByID(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "run_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "rerun_requested"})
}

func cancelWorkflowRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Actions.CancelWorkflowRunByID(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "run_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "cancelled"})
}

// ── Secrets ───────────────────────────────────────────────────────

func listRepoSecrets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	secrets, _, err := g.client.Actions.ListRepoSecrets(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(secrets)
}

func listArtifacts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListArtifactsOptions{ListOptions: listOpts(args)}
	artifacts, _, err := g.client.Actions.ListArtifacts(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(artifacts)
}

func listEnvironmentSecrets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	repoID, err := g.repoID(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	secrets, _, err := g.client.Actions.ListEnvSecrets(ctx, repoID, argStr(args, "environment"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(secrets)
}

func listOrgSecrets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	secrets, _, err := g.client.Actions.ListOrgSecrets(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(secrets)
}

// ── Checks ────────────────────────────────────────────────────────

func listCheckRuns(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListCheckRunsOptions{ListOptions: listOpts(args)}
	result, _, err := g.client.Checks.ListCheckRunsForRef(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "ref"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

func getCheckRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	run, _, err := g.client.Checks.GetCheckRun(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "check_run_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(run)
}

func listCheckSuites(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListCheckSuiteOptions{ListOptions: listOpts(args)}
	result, _, err := g.client.Checks.ListCheckSuitesForRef(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "ref"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(result)
}

// helper to get repo numeric ID for env secrets API
func (g *integration) repoID(ctx context.Context, owner, repo string) (int, error) {
	r, _, err := g.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return 0, fmt.Errorf("failed to get repo ID: %w", err)
	}
	return int(r.GetID()), nil
}
