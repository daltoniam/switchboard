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
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMetricAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	alertRuleID := r.Str("alert_rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/organizations/%s/alert-rules/%s/", s.org(args), alertRuleID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMetricAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	alertRuleID := r.Str("alert_rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/organizations/%s/alert-rules/%s/", s.org(args), alertRuleID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listIssueAlerts(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/rules/", s.org(args), project)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getIssueAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	alertRuleID := r.Str("alert_rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/rules/%s/", s.org(args), project, alertRuleID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteIssueAlert(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	alertRuleID := r.Str("alert_rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/projects/%s/%s/rules/%s/", s.org(args), project, alertRuleID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Monitors (Cron) ──────────────────────────────────────────────────

func listMonitors(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{"cursor": cursor}
	if project != "" {
		params["project"] = project
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/monitors/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMonitor(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	monitorID := r.Str("monitor_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/projects/%s/%s/monitors/%s/", s.org(args), project, monitorID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMonitor(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	project := r.Str("project")
	monitorID := r.Str("monitor_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/projects/%s/%s/monitors/%s/", s.org(args), project, monitorID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Discover ─────────────────────────────────────────────────────────

func listSavedQueries(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cursor := r.Str("cursor")
	sortBy := r.Str("sortBy")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"cursor": cursor,
		"sortBy": sortBy,
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/discover/saved/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSavedQuery(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queryID := r.Str("query_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/organizations/%s/discover/saved/%s/", s.org(args), queryID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteSavedQuery(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	queryID := r.Str("query_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/organizations/%s/discover/saved/%s/", s.org(args), queryID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Replays ──────────────────────────────────────────────────────────

func listReplays(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	cursor := r.Str("cursor")
	statsPeriod := r.Str("statsPeriod")
	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"query":       query,
		"cursor":      cursor,
		"statsPeriod": statsPeriod,
	}
	if limit > 0 {
		params["per_page"] = fmt.Sprintf("%d", limit)
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/replays/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getReplay(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	replayID := r.Str("replay_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/organizations/%s/replays/%s/", s.org(args), replayID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteReplay(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	replayID := r.Str("replay_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/organizations/%s/replays/%s/", s.org(args), replayID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
