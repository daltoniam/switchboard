package mixpanel

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func queryInsights(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	data, err := m.doGet(ctx, "/insights", map[string]string{
		"project_id":  m.projFromArgs(args),
		"bookmark_id": argStr(args, "bookmark_id"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryFunnels(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	data, err := m.doGet(ctx, "/funnels", map[string]string{
		"project_id": m.projFromArgs(args),
		"funnel_id":  argStr(args, "funnel_id"),
		"from_date":  argStr(args, "from_date"),
		"to_date":    argStr(args, "to_date"),
		"length":     argStr(args, "length"),
		"interval":   argStr(args, "interval"),
		"unit":       argStr(args, "unit"),
		"on":         argStr(args, "on"),
		"where":      argStr(args, "where"),
		"limit":      argStr(args, "limit"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryRetention(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	data, err := m.doGet(ctx, "/retention", map[string]string{
		"project_id":     m.projFromArgs(args),
		"from_date":      argStr(args, "from_date"),
		"to_date":        argStr(args, "to_date"),
		"retention_type": argStr(args, "retention_type"),
		"born_event":     argStr(args, "born_event"),
		"event":          argStr(args, "event"),
		"born_where":     argStr(args, "born_where"),
		"where":          argStr(args, "where"),
		"interval":       argStr(args, "interval"),
		"interval_count": argStr(args, "interval_count"),
		"unit":           argStr(args, "unit"),
		"on":             argStr(args, "on"),
		"limit":          argStr(args, "limit"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func querySegmentation(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	data, err := m.doGet(ctx, "/segmentation", map[string]string{
		"project_id": m.projFromArgs(args),
		"event":      argStr(args, "event"),
		"from_date":  argStr(args, "from_date"),
		"to_date":    argStr(args, "to_date"),
		"on":         argStr(args, "on"),
		"where":      argStr(args, "where"),
		"unit":       argStr(args, "unit"),
		"type":       argStr(args, "type"),
		"limit":      argStr(args, "limit"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryEventProperties(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	data, err := m.doGet(ctx, "/events/properties", map[string]string{
		"project_id": m.projFromArgs(args),
		"event":      argStr(args, "event"),
		"name":       argStr(args, "name"),
		"from_date":  argStr(args, "from_date"),
		"to_date":    argStr(args, "to_date"),
		"values":     argStr(args, "values"),
		"type":       argStr(args, "type"),
		"unit":       argStr(args, "unit"),
		"limit":      argStr(args, "limit"),
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func queryProfiles(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	form := url.Values{}
	if v := argStr(args, "distinct_id"); v != "" {
		form.Set("distinct_id", v)
	}
	if v := argStr(args, "distinct_ids"); v != "" {
		form.Set("distinct_ids", v)
	}
	if v := argStr(args, "where"); v != "" {
		form.Set("where", v)
	}
	if v := argStr(args, "output_properties"); v != "" {
		form.Set("output_properties", v)
	}
	if v := argStr(args, "session_id"); v != "" {
		form.Set("session_id", v)
	}
	if v := argStr(args, "page"); v != "" {
		form.Set("page", v)
	}
	if v := argStr(args, "filter_by_cohort"); v != "" {
		form.Set("filter_by_cohort", v)
	}
	if argBool(args, "include_all_users") {
		form.Set("include_all_users", "true")
	}

	data, err := m.doPostForm(ctx, "/engage", map[string]string{
		"project_id": m.projFromArgs(args),
	}, form)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
