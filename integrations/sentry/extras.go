package sentry

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// ── Alerts ───────────────────────────────────────────────────────────

func listMetricAlerts(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/alert-rules/", s.org(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMetricAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/alert-rules/%s/", s.org(args), argStr(args, "alert_rule_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteMetricAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/organizations/%s/alert-rules/%s/", s.org(args), argStr(args, "alert_rule_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listIssueAlerts(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/rules/", s.org(args), argStr(args, "project"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getIssueAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/rules/%s/", s.org(args), argStr(args, "project"), argStr(args, "alert_rule_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteIssueAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/projects/%s/%s/rules/%s/", s.org(args), argStr(args, "project"), argStr(args, "alert_rule_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Monitors (Cron) ──────────────────────────────────────────────────

func listMonitors(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{"cursor": argStr(args, "cursor")}
	if v := argStr(args, "project"); v != "" {
		params["project"] = v
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/monitors/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMonitor(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/projects/%s/%s/monitors/%s/", s.org(args), argStr(args, "project"), argStr(args, "monitor_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteMonitor(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/projects/%s/%s/monitors/%s/", s.org(args), argStr(args, "project"), argStr(args, "monitor_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Discover ─────────────────────────────────────────────────────────

func listSavedQueries(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"cursor": argStr(args, "cursor"),
		"sortBy": argStr(args, "sortBy"),
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/discover/saved/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getSavedQuery(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/discover/saved/%s/", s.org(args), argStr(args, "query_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteSavedQuery(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/organizations/%s/discover/saved/%s/", s.org(args), argStr(args, "query_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Replays ──────────────────────────────────────────────────────────

func listReplays(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"query":       argStr(args, "query"),
		"cursor":      argStr(args, "cursor"),
		"statsPeriod": argStr(args, "statsPeriod"),
	}
	if v := argInt(args, "limit"); v > 0 {
		params["per_page"] = fmt.Sprintf("%d", v)
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/replays/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getReplay(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/replays/%s/", s.org(args), argStr(args, "replay_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteReplay(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/organizations/%s/replays/%s/", s.org(args), argStr(args, "replay_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

