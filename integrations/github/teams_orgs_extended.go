package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Teams Extended ────────────────────────────────────────────────

func createTeam(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	t := gh.NewTeam{
		Name:        argStr(args, "name"),
		Description: gh.Ptr(argStr(args, "description")),
		Privacy:     gh.Ptr(argStr(args, "privacy")),
	}
	if perm := argStr(args, "permission"); perm != "" {
		t.Permission = gh.Ptr(perm)
	}
	team, _, err := g.client.Teams.CreateTeam(ctx, argStr(args, "org"), t)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(team)
}

func editTeam(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	t := gh.NewTeam{Name: argStr(args, "name")}
	if v := argStr(args, "description"); v != "" {
		t.Description = gh.Ptr(v)
	}
	if v := argStr(args, "privacy"); v != "" {
		t.Privacy = gh.Ptr(v)
	}
	if v := argStr(args, "permission"); v != "" {
		t.Permission = gh.Ptr(v)
	}
	team, _, err := g.client.Teams.EditTeamBySlug(ctx, argStr(args, "org"), argStr(args, "slug"), t, false)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(team)
}

func deleteTeam(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Teams.DeleteTeamBySlug(ctx, argStr(args, "org"), argStr(args, "slug"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "deleted"})
}

func addTeamMember(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.TeamAddTeamMembershipOptions{}
	if role := argStr(args, "role"); role != "" {
		opts.Role = role
	}
	membership, _, err := g.client.Teams.AddTeamMembershipBySlug(ctx, argStr(args, "org"), argStr(args, "slug"), argStr(args, "username"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(membership)
}

func removeTeamMember(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Teams.RemoveTeamMembershipBySlug(ctx, argStr(args, "org"), argStr(args, "slug"), argStr(args, "username"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "removed"})
}

func addTeamRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.TeamAddTeamRepoOptions{}
	if perm := argStr(args, "permission"); perm != "" {
		opts.Permission = perm
	}
	_, err := g.client.Teams.AddTeamRepoBySlug(ctx, argStr(args, "org"), argStr(args, "slug"), argStr(args, "owner"), argStr(args, "repo"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "added"})
}

func removeTeamRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.client.Teams.RemoveTeamRepoBySlug(ctx, argStr(args, "org"), argStr(args, "slug"), argStr(args, "owner"), argStr(args, "repo"))
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "removed"})
}

// ── Organizations Extended ────────────────────────────────────────

func listPendingOrgInvitations(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOptions{Page: listOpts(args).Page, PerPage: listOpts(args).PerPage}
	invitations, _, err := g.client.Organizations.ListPendingOrgInvitations(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(invitations)
}

func listOutsideCollaborators(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	opts := &gh.ListOutsideCollaboratorsOptions{ListOptions: listOpts(args)}
	if filter := argStr(args, "filter"); filter != "" {
		opts.Filter = filter
	}
	users, _, err := g.client.Organizations.ListOutsideCollaborators(ctx, argStr(args, "org"), opts)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(users)
}
