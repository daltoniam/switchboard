package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Search Extended ───────────────────────────────────────────────

func searchTopics(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SearchOptions{ListOptions: listOpts(args)}
	resp, _, err := g.client.Search.Topics(ctx, argStr(args, "query"), opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Topics)
}

func searchLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SearchOptions{
		Sort:        argStr(args, "sort"),
		Order:       argStr(args, "order"),
		ListOptions: listOpts(args),
	}
	resp, _, err := g.client.Search.Labels(ctx, argInt64(args, "repository_id"), argStr(args, "query"), opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Labels)
}

// ── Security Extended ─────────────────────────────────────────────

func getSecretScanningAlert(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	alert, _, err := g.client.SecretScanning.GetAlert(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "alert_number"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alert)
}

func getDependabotAlert(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	alert, _, err := g.client.Dependabot.GetRepoAlert(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt(args, "alert_number"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alert)
}

func listCodeScanningAnalyses(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.AnalysesListOptions{
		Ref: gh.Ptr(argStr(args, "ref")),
		ListOptions: gh.ListOptions{
			Page:    listOpts(args).Page,
			PerPage: listOpts(args).PerPage,
		},
	}
	analyses, _, err := g.client.CodeScanning.ListAnalysesForRepo(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(analyses)
}

// ── SBOM ──────────────────────────────────────────────────────────

func getSBOM(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	sbom, _, err := g.client.DependencyGraph.GetSBOM(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(sbom)
}

// ── Activity Extended ─────────────────────────────────────────────

func markNotificationsRead(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Activity.MarkNotificationsRead(ctx, gh.Timestamp{})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "marked_read"})
}

func starRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Activity.Star(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "starred"})
}

func unstarRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Activity.Unstar(ctx, argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "unstarred"})
}

func listStarred(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ActivityListStarredOptions{
		Sort:        argStr(args, "sort"),
		Direction:   argStr(args, "direction"),
		ListOptions: listOpts(args),
	}
	repos, _, err := g.client.Activity.ListStarred(ctx, argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(repos)
}
