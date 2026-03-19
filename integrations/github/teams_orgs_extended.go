package github

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
	gh "github.com/google/go-github/v68/github"
)

// ── Teams Extended ────────────────────────────────────────────────

func createTeam(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	name := r.Str("name")
	description := r.Str("description")
	privacy := r.Str("privacy")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	t := gh.NewTeam{
		Name:        name,
		Description: gh.Ptr(description),
		Privacy:     gh.Ptr(privacy),
	}
	if perm, err := mcp.ArgStr(args, "permission"); err != nil {
		return mcp.ErrResult(err)
	} else if perm != "" {
		t.Permission = gh.Ptr(perm)
	}
	team, _, err := g.client.Teams.CreateTeam(ctx, org, t)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(team)
}

func editTeam(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	t := gh.NewTeam{Name: name}
	if v, err := mcp.ArgStr(args, "description"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		t.Description = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "privacy"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		t.Privacy = gh.Ptr(v)
	}
	if v, err := mcp.ArgStr(args, "permission"); err != nil {
		return mcp.ErrResult(err)
	} else if v != "" {
		t.Permission = gh.Ptr(v)
	}
	team, _, err := g.client.Teams.EditTeamBySlug(ctx, org, slug, t, false)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(team)
}

func deleteTeam(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Teams.DeleteTeamBySlug(ctx, org, slug)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted"})
}

func addTeamMember(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	username := r.Str("username")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.TeamAddTeamMembershipOptions{}
	if role, err := mcp.ArgStr(args, "role"); err != nil {
		return mcp.ErrResult(err)
	} else if role != "" {
		opts.Role = role
	}
	membership, _, err := g.client.Teams.AddTeamMembershipBySlug(ctx, org, slug, username, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(membership)
}

func removeTeamMember(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	username := r.Str("username")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Teams.RemoveTeamMembershipBySlug(ctx, org, slug, username)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

func addTeamRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.TeamAddTeamRepoOptions{}
	if perm, err := mcp.ArgStr(args, "permission"); err != nil {
		return mcp.ErrResult(err)
	} else if perm != "" {
		opts.Permission = perm
	}
	_, err := g.client.Teams.AddTeamRepoBySlug(ctx, org, slug, owner, repo, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "added"})
}

func removeTeamRepo(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	org := r.Str("org")
	slug := r.Str("slug")
	owner := r.Str("owner")
	repo := r.Str("repo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.client.Teams.RemoveTeamRepoBySlug(ctx, org, slug, owner, repo)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "removed"})
}

// ── Organizations Extended ────────────────────────────────────────

func listPendingOrgInvitations(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	org, err := mcp.ArgStr(args, "org")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOptions{Page: lo.Page, PerPage: lo.PerPage}
	invitations, _, err := g.client.Organizations.ListPendingOrgInvitations(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(invitations)
}

func listOutsideCollaborators(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	lo, err := listOpts(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	org, err := mcp.ArgStr(args, "org")
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &gh.ListOutsideCollaboratorsOptions{ListOptions: lo}
	if filter, err := mcp.ArgStr(args, "filter"); err != nil {
		return mcp.ErrResult(err)
	} else if filter != "" {
		opts.Filter = filter
	}
	users, _, err := g.client.Organizations.ListOutsideCollaborators(ctx, org, opts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(users)
}
