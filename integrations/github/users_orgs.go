package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Users ─────────────────────────────────────────────────────────

func getAuthenticatedUser(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	user, _, err := g.client.Users.Get(ctx, "")
	if err != nil {
		return errResult(err)
	}
	return jsonResult(user)
}

func getUser(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	user, _, err := g.client.Users.Get(ctx, argStr(args, "username"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(user)
}

func listUserFollowers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	users, _, err := g.client.Users.ListFollowers(ctx, argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(users)
}

func listUserFollowing(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	users, _, err := g.client.Users.ListFollowing(ctx, argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(users)
}

func listUserKeys(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	keys, _, err := g.client.Users.ListKeys(ctx, argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(keys)
}

// ── Organizations ─────────────────────────────────────────────────

func getOrg(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	org, _, err := g.client.Organizations.Get(ctx, argStr(args, "org"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(org)
}

func listUserOrgs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	orgs, _, err := g.client.Organizations.List(ctx, argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(orgs)
}

func listOrgMembers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListMembersOptions{
		PublicOnly:  false,
		ListOptions: listOpts(args),
	}
	if role := argStr(args, "role"); role != "" {
		opts.Role = role
	}
	members, _, err := g.client.Organizations.ListMembers(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(members)
}

func listOrgTeams(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	teams, _, err := g.client.Teams.ListTeams(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(teams)
}

func getTeamBySlug(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	team, _, err := g.client.Teams.GetTeamBySlug(ctx, argStr(args, "org"), argStr(args, "slug"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(team)
}

func listTeamMembers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.TeamListTeamMembersOptions{ListOptions: listOpts(args)}
	if role := argStr(args, "role"); role != "" {
		opts.Role = role
	}
	members, _, err := g.client.Teams.ListTeamMembersBySlug(ctx, argStr(args, "org"), argStr(args, "slug"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(members)
}

func listTeamRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	repos, _, err := g.client.Teams.ListTeamReposBySlug(ctx, argStr(args, "org"), argStr(args, "slug"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(repos)
}
