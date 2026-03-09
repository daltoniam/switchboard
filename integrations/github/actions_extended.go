package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Workflows Extended ────────────────────────────────────────────

func triggerWorkflow(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	event := gh.CreateWorkflowDispatchEventRequest{
		Ref: argStr(args, "ref"),
	}
	_, err := g.client.Actions.CreateWorkflowDispatchEventByFileName(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "workflow_id"), event)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "dispatched"})
}

func rerunFailedJobs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Actions.RerunFailedJobsByID(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "run_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "rerun_failed_requested"})
}

func getWorkflowJob(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	job, _, err := g.client.Actions.GetWorkflowJobByID(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "job_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(job)
}

func getWorkflowJobLogs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	url, _, err := g.client.Actions.GetWorkflowJobLogs(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "job_id"), 0)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"logs_url": url.String()})
}

func deleteWorkflowRun(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Actions.DeleteWorkflowRun(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "run_id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

// ── Actions Variables ─────────────────────────────────────────────

func listRepoVariables(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	resp, _, err := g.client.Actions.ListRepoVariables(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp.Variables)
}

func createRepoVariable(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	v := &gh.ActionsVariable{
		Name:  argStr(args, "name"),
		Value: argStr(args, "value"),
	}
	_, err := g.client.Actions.CreateRepoVariable(ctx, argStr(args, "owner"), argStr(args, "repo"), v)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "created", "name": v.Name})
}

func updateRepoVariable(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	v := &gh.ActionsVariable{
		Name:  argStr(args, "name"),
		Value: argStr(args, "value"),
	}
	_, err := g.client.Actions.UpdateRepoVariable(ctx, argStr(args, "owner"), argStr(args, "repo"), v)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "updated", "name": v.Name})
}

func deleteRepoVariable(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Actions.DeleteRepoVariable(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "name"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

func listOrgVariables(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	resp, _, err := g.client.Actions.ListOrgVariables(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp.Variables)
}

func listEnvVariables(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	resp, _, err := g.client.Actions.ListEnvVariables(ctx, argStr(args, "owner"), argStr(args, "repo"), argStr(args, "environment"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp.Variables)
}

// ── Runners ───────────────────────────────────────────────────────

func listRunners(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListRunnersOptions{ListOptions: listOpts(args)}
	resp, _, err := g.client.Actions.ListRunners(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp.Runners)
}

func listOrgRunners(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListRunnersOptions{ListOptions: listOpts(args)}
	resp, _, err := g.client.Actions.ListOrganizationRunners(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(resp.Runners)
}
