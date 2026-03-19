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
	return mcp.JSONResult(user)
}

func getUser(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	username, err := mcp.ArgStr(args, "username")
	if err != nil {
		return mcp.ErrResult(err)
	}
	user, _, err := g.client.Users.Get(ctx, username)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(user)
}

func listUserFollowers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	username, err := mcp.ArgStr(args, "username")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	users, _, err := g.client.Users.ListFollowers(ctx, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(users)
}

func listUserFollowing(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	username, err := mcp.ArgStr(args, "username")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	users, _, err := g.client.Users.ListFollowing(ctx, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(users)
}

func listUserKeys(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	username, err := mcp.ArgStr(args, "username")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	keys, _, err := g.client.Users.ListKeys(ctx, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(keys)
}

// ── Organizations ─────────────────────────────────────────────────

func getOrg(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	org, err := mcp.ArgStr(args, "org")
	if err != nil {
		return mcp.ErrResult(err)
	}
	result, _, err := g.client.Organizations.Get(ctx, org)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(result)
}

func listUserOrgs(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	username, err := mcp.ArgStr(args, "username")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	orgs, _, err := g.client.Organizations.List(ctx, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(orgs)
}

func listOrgMembers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	org, err := mcp.ArgStr(args, "org")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListMembersOptions{
		PublicOnly:  false,
		ListOptions: lo,
	}
	if role, err := mcp.ArgStr(args, "role"); err != nil {
		return mcp.ErrResult(err)
	} else if role != "" {
		opts.Role = role
	}
	members, _, err := g.client.Organizations.ListMembers(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(members)
}

func listOrgTeams(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	org, err := mcp.ArgStr(args, "org")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	teams, _, err := g.client.Teams.ListTeams(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(teams)
}

func getTeamBySlug(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	team, _, err := g.client.Teams.GetTeamBySlug(ctx, org, slug)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(team)
}

func listTeamMembers(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.TeamListTeamMembersOptions{ListOptions: lo}
	if role, err := mcp.ArgStr(args, "role"); err != nil {
		return mcp.ErrResult(err)
	} else if role != "" {
		opts.Role = role
	}
	members, _, err := g.client.Teams.ListTeamMembersBySlug(ctx, org, slug, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(members)
}

func listTeamRepos(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	repos, _, err := g.client.Teams.ListTeamReposBySlug(ctx, org, slug, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(repos)
}
