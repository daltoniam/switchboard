package sentry

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// ── Organizations ────────────────────────────────────────────────────

func getOrganization(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/", s.org(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOrgProjects(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/organizations/%s/projects/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOrgTeams(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/organizations/%s/teams/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOrgMembers(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/organizations/%s/members/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getOrgMember(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	memberID := r.Str("member_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/organizations/%s/members/%s/", s.org(args), memberID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOrgRepos(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/organizations/%s/repos/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func resolveShortID(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	shortID := r.Str("short_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/organizations/%s/shortids/%s/", s.org(args), shortID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Teams ────────────────────────────────────────────────────────────

func getTeam(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/teams/%s/%s/", s.org(args), team)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTeam(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	slug := r.Str("slug")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]string{"name": name}
	if slug != "" {
		body["slug"] = slug
	}
	path := fmt.Sprintf("/organizations/%s/teams/", s.org(args))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTeam(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/teams/%s/%s/", s.org(args), team)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTeamProjects(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	team := r.Str("team")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/teams/%s/%s/projects/%s", s.org(args), team, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
