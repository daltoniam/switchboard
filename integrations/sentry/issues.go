package sentry

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listProjects(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
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

func getProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/", s.org(args), project)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	name := r.Str("name")
	slug := r.Str("slug")
	platform := r.Str("platform")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if slug != "" {
		body["slug"] = slug
	}
	if platform != "" {
		body["platform"] = platform
	}
	if _, ok := args["isBookmarked"]; ok {
		v, err := mcp.ArgBool(args, "isBookmarked")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["isBookmarked"] = v
	}
	path := fmt.Sprintf("/projects/%s/%s/", s.org(args), project)
	data, err := s.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/projects/%s/%s/", s.org(args), project)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProject(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	slug := r.Str("slug")
	platform := r.Str("platform")
	team := r.Str("team")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]string{"name": name}
	if slug != "" {
		body["slug"] = slug
	}
	if platform != "" {
		body["platform"] = platform
	}
	path := fmt.Sprintf("/teams/%s/%s/projects/", s.org(args), team)
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectKeys(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/keys/", s.org(args), project)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectEnvironments(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/environments/", s.org(args), project)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectTags(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/tags/", s.org(args), project)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getProjectStats(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	stat := r.Str("stat")
	since := r.Str("since")
	until := r.Str("until")
	resolution := r.Str("resolution")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"stat":       stat,
		"since":      since,
		"until":      until,
		"resolution": resolution,
	})
	data, err := s.get(ctx, "/projects/%s/%s/stats/%s", s.org(args), project, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectHooks(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/hooks/", s.org(args), project)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Issues & Events ──────────────────────────────────────────────────

func listIssues(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	cursor := r.Str("cursor")
	sort := r.Str("sort")
	statsPeriod := r.Str("statsPeriod")
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if query == "" {
		query = "is:unresolved"
	}
	q := queryEncode(map[string]string{
		"query":       query,
		"cursor":      cursor,
		"sort":        sort,
		"statsPeriod": statsPeriod,
	})
	data, err := s.get(ctx, "/projects/%s/%s/issues/%s", s.org(args), project, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIssue(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueID := r.Str("issue_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/issues/%s/", issueID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateIssue(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueID := r.Str("issue_id")
	status := r.Str("status")
	assignedTo := r.Str("assignedTo")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if status != "" {
		body["status"] = status
	}
	if assignedTo != "" {
		body["assignedTo"] = assignedTo
	}
	if _, ok := args["hasSeen"]; ok {
		v, err := mcp.ArgBool(args, "hasSeen")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["hasSeen"] = v
	}
	if _, ok := args["isBookmarked"]; ok {
		v, err := mcp.ArgBool(args, "isBookmarked")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["isBookmarked"] = v
	}
	if _, ok := args["isSubscribed"]; ok {
		v, err := mcp.ArgBool(args, "isSubscribed")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["isSubscribed"] = v
	}
	if _, ok := args["isPublic"]; ok {
		v, err := mcp.ArgBool(args, "isPublic")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["isPublic"] = v
	}
	path := fmt.Sprintf("/issues/%s/", issueID)
	data, err := s.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIssue(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueID := r.Str("issue_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/issues/%s/", issueID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueEvents(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueID := r.Str("issue_id")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/issues/%s/events/%s", issueID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueHashes(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueID := r.Str("issue_id")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/issues/%s/hashes/%s", issueID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIssueTagValues(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	issueID := r.Str("issue_id")
	tagName := r.Str("tag_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/issues/%s/tags/%s/values/", issueID, tagName)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listProjectEvents(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/projects/%s/%s/events/%s", s.org(args), project, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getEvent(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	eventID := r.Str("event_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/events/%s/", s.org(args), project, eventID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listOrgIssues(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	cursor := r.Str("cursor")
	sort := r.Str("sort")
	statsPeriod := r.Str("statsPeriod")
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"query":       query,
		"cursor":      cursor,
		"sort":        sort,
		"statsPeriod": statsPeriod,
	}
	if project != "" {
		params["project"] = project
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/issues/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
