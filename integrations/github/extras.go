package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Gists ─────────────────────────────────────────────────────────

func listGists(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.GistListOptions{ListOptions: listOpts(args)}
	gists, _, err := g.client.Gists.List(ctx, argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(gists)
}

func getGist(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	gist, _, err := g.client.Gists.Get(ctx, argStr(args, "id"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(gist)
}

func createGist(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	gist := &gh.Gist{
		Description: gh.Ptr(argStr(args, "description")),
		Public:      gh.Ptr(argBool(args, "public")),
		Files: map[gh.GistFilename]gh.GistFile{
			gh.GistFilename(argStr(args, "filename")): {Content: gh.Ptr(argStr(args, "content"))},
		},
	}
	g2, _, err := g.client.Gists.Create(ctx, gist)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(g2)
}

// ── Activity ──────────────────────────────────────────────────────

func listStargazers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	stargazers, _, err := g.client.Activity.ListStargazers(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(stargazers)
}

func listWatchers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	watchers, _, err := g.client.Activity.ListWatchers(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(watchers)
}

func listNotifications(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.NotificationListOptions{
		All:           argBool(args, "all"),
		Participating: argBool(args, "participating"),
		ListOptions:   listOpts(args),
	}
	notifications, _, err := g.client.Activity.ListNotifications(ctx, opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(notifications)
}

func listRepoEvents(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	events, _, err := g.client.Activity.ListRepositoryEvents(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(events)
}

// ── Code Scanning ─────────────────────────────────────────────────

func listCodeScanningAlerts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.AlertListOptions{
		State: argStr(args, "state"),
		Ref:   argStr(args, "ref"),
		ListOptions: gh.ListOptions{
			Page:    listOpts(args).Page,
			PerPage: listOpts(args).PerPage,
		},
	}
	alerts, _, err := g.client.CodeScanning.ListAlertsForRepo(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(alerts)
}

func getCodeScanningAlert(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	alert, _, err := g.client.CodeScanning.GetAlert(ctx, argStr(args, "owner"), argStr(args, "repo"), argInt64(args, "alert_number"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(alert)
}

// ── Secret Scanning ───────────────────────────────────────────────

func listSecretScanningAlerts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.SecretScanningAlertListOptions{
		State:       argStr(args, "state"),
		SecretType:  argStr(args, "secret_type"),
		ListOptions: listOpts(args),
	}
	alerts, _, err := g.client.SecretScanning.ListAlertsForRepo(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(alerts)
}

// ── Dependabot ────────────────────────────────────────────────────

func listDependabotAlerts(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListAlertsOptions{
		State:       gh.Ptr(argStr(args, "state")),
		Severity:    gh.Ptr(argStr(args, "severity")),
		ListOptions: listOpts(args),
	}
	alerts, _, err := g.client.Dependabot.ListRepoAlerts(ctx, argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(alerts)
}

// ── Copilot ───────────────────────────────────────────────────────

func getCopilotOrgUsage(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	billing, _, err := g.client.Copilot.GetCopilotBilling(ctx, argStr(args, "org"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(billing)
}
