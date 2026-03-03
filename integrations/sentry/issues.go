package sentry

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/organizations/%s/projects/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/", s.org(args), argStr(args, "project"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "slug"); v != "" {
		body["slug"] = v
	}
	if v := argStr(args, "platform"); v != "" {
		body["platform"] = v
	}
	if _, ok := args["isBookmarked"]; ok {
		body["isBookmarked"] = argBool(args, "isBookmarked")
	}
	path := fmt.Sprintf("/projects/%s/%s/", s.org(args), argStr(args, "project"))
	data, err := s.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/projects/%s/%s/", s.org(args), argStr(args, "project"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]string{"name": argStr(args, "name")}
	if v := argStr(args, "slug"); v != "" {
		body["slug"] = v
	}
	if v := argStr(args, "platform"); v != "" {
		body["platform"] = v
	}
	path := fmt.Sprintf("/teams/%s/%s/projects/", s.org(args), argStr(args, "team"))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listProjectKeys(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/keys/", s.org(args), argStr(args, "project"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listProjectEnvironments(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/environments/", s.org(args), argStr(args, "project"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listProjectTags(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/tags/", s.org(args), argStr(args, "project"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getProjectStats(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"stat":       argStr(args, "stat"),
		"since":      argStr(args, "since"),
		"until":      argStr(args, "until"),
		"resolution": argStr(args, "resolution"),
	})
	data, err := s.get(ctx, "/projects/%s/%s/stats/%s", s.org(args), argStr(args, "project"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listProjectHooks(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/hooks/", s.org(args), argStr(args, "project"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Issues & Events ──────────────────────────────────────────────────

func listIssues(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	query := argStr(args, "query")
	if query == "" {
		query = "is:unresolved"
	}
	q := queryEncode(map[string]string{
		"query":       query,
		"cursor":      argStr(args, "cursor"),
		"sort":        argStr(args, "sort"),
		"statsPeriod": argStr(args, "statsPeriod"),
	})
	data, err := s.get(ctx, "/projects/%s/%s/issues/%s", s.org(args), argStr(args, "project"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getIssue(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/issues/%s/", argStr(args, "issue_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateIssue(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "status"); v != "" {
		body["status"] = v
	}
	if v := argStr(args, "assignedTo"); v != "" {
		body["assignedTo"] = v
	}
	if _, ok := args["hasSeen"]; ok {
		body["hasSeen"] = argBool(args, "hasSeen")
	}
	if _, ok := args["isBookmarked"]; ok {
		body["isBookmarked"] = argBool(args, "isBookmarked")
	}
	if _, ok := args["isSubscribed"]; ok {
		body["isSubscribed"] = argBool(args, "isSubscribed")
	}
	if _, ok := args["isPublic"]; ok {
		body["isPublic"] = argBool(args, "isPublic")
	}
	path := fmt.Sprintf("/issues/%s/", argStr(args, "issue_id"))
	data, err := s.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteIssue(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/issues/%s/", argStr(args, "issue_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listIssueEvents(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/issues/%s/events/%s", argStr(args, "issue_id"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listIssueHashes(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/issues/%s/hashes/%s", argStr(args, "issue_id"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getIssueTagValues(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/issues/%s/tags/%s/values/", argStr(args, "issue_id"), argStr(args, "tag_name"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listProjectEvents(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/projects/%s/%s/events/%s", s.org(args), argStr(args, "project"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getEvent(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/events/%s/", s.org(args), argStr(args, "project"), argStr(args, "event_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listOrgIssues(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"query":       argStr(args, "query"),
		"cursor":      argStr(args, "cursor"),
		"sort":        argStr(args, "sort"),
		"statsPeriod": argStr(args, "statsPeriod"),
	}
	if v := argStr(args, "project"); v != "" {
		params["project"] = v
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/issues/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

