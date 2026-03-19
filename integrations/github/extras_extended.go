package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Search Extended ───────────────────────────────────────────────

func searchTopics(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	query, err := mcp.ArgStr(args, "query")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SearchOptions{ListOptions: lo}
	resp, _, err := g.client.Search.Topics(ctx, query, opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Topics)
}

func searchLabels(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	repositoryID := r.Int64("repository_id")
	query := r.Str("query")
	sort := r.Str("sort")
	order := r.Str("order")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SearchOptions{
		Sort:        sort,
		Order:       order,
		ListOptions: lo,
	}
	resp, _, err := g.client.Search.Labels(ctx, repositoryID, query, opts)
	if err != nil {
		return errResult(err)
	}
	return searchResult(resp.GetTotal(), resp.Labels)
}

// ── Security Extended ─────────────────────────────────────────────

func getSecretScanningAlert(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	alertNumber := r.Int64("alert_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	alert, _, err := g.client.SecretScanning.GetAlert(ctx, owner, repo, alertNumber)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alert)
}

func getDependabotAlert(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	alertNumber := r.Int("alert_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	alert, _, err := g.client.Dependabot.GetRepoAlert(ctx, owner, repo, alertNumber)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alert)
}

func listCodeScanningAnalyses(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	ref := r.Str("ref")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.AnalysesListOptions{
		Ref: gh.Ptr(ref),
		ListOptions: gh.ListOptions{
			Page:    lo.Page,
			PerPage: lo.PerPage,
		},
	}
	analyses, _, err := g.client.CodeScanning.ListAnalysesForRepo(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(analyses)
}

// ── SBOM ──────────────────────────────────────────────────────────

func getSBOM(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	sbom, _, err := g.client.DependencyGraph.GetSBOM(ctx, owner, repo)
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
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Activity.Star(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "starred"})
}

func unstarRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Activity.Unstar(ctx, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "unstarred"})
}

func listStarred(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	username := r.Str("username")
	sort := r.Str("sort")
	direction := r.Str("direction")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ActivityListStarredOptions{
		Sort:        sort,
		Direction:   direction,
		ListOptions: lo,
	}
	repos, _, err := g.client.Activity.ListStarred(ctx, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(repos)
}
