package launchdarkly

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func requireKey(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func queryFilter(query string) string {
	if query == "" {
		return ""
	}
	return "query:" + query
}

func listProjects(ctx context.Context, l *launchdarkly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	limit := r.OptInt("limit", defaultLimit)
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	vals := paginationQuery(limit, offset)
	setIfNotEmpty(vals, "filter", queryFilter(query))
	data, err := l.get(ctx, "/projects?%s", vals.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listEnvironments(ctx context.Context, l *launchdarkly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	query := r.Str("query")
	limit := r.OptInt("limit", defaultLimit)
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("project_key", projectKey); err != nil {
		return mcp.ErrResult(err)
	}
	vals := paginationQuery(limit, offset)
	setIfNotEmpty(vals, "filter", queryFilter(query))
	data, err := l.get(ctx, "/projects/%s/environments?%s", url.PathEscape(projectKey), vals.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listFlags(ctx context.Context, l *launchdarkly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	envKey := r.Str("environment_key")
	query := r.Str("query")
	tag := r.Str("tag")
	sort := r.Str("sort")
	limit := r.OptInt("limit", defaultLimit)
	offset := r.Int("offset")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("project_key", projectKey); err != nil {
		return mcp.ErrResult(err)
	}
	vals := paginationQuery(limit, offset)
	setIfNotEmpty(vals, "env", envKey)
	setIfNotEmpty(vals, "filter", queryFilter(query))
	setIfNotEmpty(vals, "tag", tag)
	setIfNotEmpty(vals, "sort", sort)
	data, err := l.get(ctx, "/flags/%s?%s", url.PathEscape(projectKey), vals.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getFlag(ctx context.Context, l *launchdarkly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	flagKey := r.Str("flag_key")
	envKey := r.Str("environment_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("project_key", projectKey); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("flag_key", flagKey); err != nil {
		return mcp.ErrResult(err)
	}
	vals := url.Values{}
	setIfNotEmpty(vals, "env", envKey)
	path := fmt.Sprintf("/flags/%s/%s", url.PathEscape(projectKey), url.PathEscape(flagKey))
	if len(vals) > 0 {
		path += "?" + vals.Encode()
	}
	data, err := l.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listFlagStatuses(ctx context.Context, l *launchdarkly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	envKey := r.Str("environment_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("project_key", projectKey); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("environment_key", envKey); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := l.get(ctx, "/flag-statuses/%s/%s", url.PathEscape(projectKey), url.PathEscape(envKey))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func toggleFlag(ctx context.Context, l *launchdarkly, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	projectKey := r.Str("project_key")
	flagKey := r.Str("flag_key")
	envKey := r.Str("environment_key")
	comment := r.Str("comment")
	on := r.Bool("on")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("project_key", projectKey); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("flag_key", flagKey); err != nil {
		return mcp.ErrResult(err)
	}
	if err := requireKey("environment_key", envKey); err != nil {
		return mcp.ErrResult(err)
	}
	if v, ok := args["on"]; !ok || v == nil {
		return mcp.ErrResult(fmt.Errorf("on is required (true or false)"))
	}
	kind := "turnFlagOff"
	if on {
		kind = "turnFlagOn"
	}
	body := map[string]any{
		"environmentKey": envKey,
		"instructions":   []map[string]any{{"kind": kind}},
	}
	if comment != "" {
		body["comment"] = comment
	}
	path := fmt.Sprintf("/flags/%s/%s", url.PathEscape(projectKey), url.PathEscape(flagKey))
	data, err := l.semanticPatch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
