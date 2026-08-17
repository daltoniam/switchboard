package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Gists ─────────────────────────────────────────────────────────

func listGists(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	username := r.Str("username")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.GistListOptions{ListOptions: gh.ListOptions{Page: page, PerPage: perPage}}
	gists, _, err := g.client.Gists.List(ctx, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(gists)
}

func getGist(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	gist, _, err := g.client.Gists.Get(ctx, id)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(gist)
}

func createGist(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	description := r.Str("description")
	public := r.Bool("public")
	filename := r.Str("filename")
	content := r.Str("content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	gist := &gh.Gist{
		Description: gh.Ptr(description),
		Public:      gh.Ptr(public),
		Files: map[gh.GistFilename]gh.GistFile{
			gh.GistFilename(filename): {Content: gh.Ptr(content)},
		},
	}
	g2, _, err := g.client.Gists.Create(ctx, gist)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(g2)
}

// ── Activity ──────────────────────────────────────────────────────

func listStargazers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	stargazers, _, err := g.client.Activity.ListStargazers(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(stargazers)
}

func listWatchers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	watchers, _, err := g.client.Activity.ListWatchers(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(watchers)
}

func listNotifications(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	all := r.Bool("all")
	participating := r.Bool("participating")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.NotificationListOptions{
		All:           all,
		Participating: participating,
		ListOptions:   gh.ListOptions{Page: page, PerPage: perPage},
	}
	notifications, _, err := g.client.Activity.ListNotifications(ctx, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(notifications)
}

func listRepoEvents(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: page, PerPage: perPage}
	events, _, err := g.client.Activity.ListRepositoryEvents(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(events)
}

// ── Code Scanning ─────────────────────────────────────────────────

func listCodeScanningAlerts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	state := r.Str("state")
	ref := r.Str("ref")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.AlertListOptions{
		State: state,
		Ref:   ref,
		ListOptions: gh.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}
	alerts, _, err := g.client.CodeScanning.ListAlertsForRepo(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alerts)
}

func getCodeScanningAlert(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	alertNumber := r.Int64("alert_number")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	alert, _, err := g.client.CodeScanning.GetAlert(ctx, owner, repo, alertNumber)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alert)
}

// ── Secret Scanning ───────────────────────────────────────────────

func listSecretScanningAlerts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	state := r.Str("state")
	secretType := r.Str("secret_type")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.SecretScanningAlertListOptions{
		State:       state,
		SecretType:  secretType,
		ListOptions: gh.ListOptions{Page: page, PerPage: perPage},
	}
	alerts, _, err := g.client.SecretScanning.ListAlertsForRepo(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alerts)
}

// ── Dependabot ────────────────────────────────────────────────────

func listDependabotAlerts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	owner := r.Str("owner")
	repo := r.Str("repo")
	state := r.Str("state")
	severity := r.Str("severity")
	page := r.OptInt("page", 1)
	perPage := r.OptInt("per_page", 10)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListAlertsOptions{
		State:       gh.Ptr(state),
		Severity:    gh.Ptr(severity),
		ListOptions: gh.ListOptions{Page: page, PerPage: perPage},
	}
	alerts, _, err := g.client.Dependabot.ListRepoAlerts(ctx, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(alerts)
}

// ── Copilot ───────────────────────────────────────────────────────

func getCopilotOrgUsage(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	org, err := mcp.ArgStr(args, "org")
	if err != nil {
		return mcp.ErrResult(err)
	}
	billing, _, err := g.client.Copilot.GetCopilotBilling(ctx, org)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(billing)
}
