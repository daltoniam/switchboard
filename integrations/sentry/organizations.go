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
		return errResult(err)
	}
	return rawResult(data)
}

func listOrgProjects(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/organizations/%s/projects/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listOrgTeams(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/organizations/%s/teams/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listOrgMembers(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/organizations/%s/members/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getOrgMember(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/members/%s/", s.org(args), argStr(args, "member_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listOrgRepos(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/organizations/%s/repos/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func resolveShortID(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/shortids/%s/", s.org(args), argStr(args, "short_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Teams ────────────────────────────────────────────────────────────

func getTeam(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/teams/%s/%s/", s.org(args), argStr(args, "team"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createTeam(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]string{"name": argStr(args, "name")}
	if v := argStr(args, "slug"); v != "" {
		body["slug"] = v
	}
	path := fmt.Sprintf("/organizations/%s/teams/", s.org(args))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteTeam(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/teams/%s/%s/", s.org(args), argStr(args, "team"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listTeamProjects(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/teams/%s/%s/projects/%s", s.org(args), argStr(args, "team"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
