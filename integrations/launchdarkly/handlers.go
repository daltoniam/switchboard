package launchdarkly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// environmentSecretFields are per-environment flag config keys that carry
// LaunchDarkly hashing secrets or UI-only noise. Compaction cannot reach
// them (environments is a dynamic-key map), so handlers strip them before
// the document leaves the adapter.
var environmentSecretFields = []string{"salt", "sel", "_site"}

func scrubEnvironments(flag map[string]any) {
	envs, ok := flag["environments"].(map[string]any)
	if !ok {
		return
	}
	for _, cfg := range envs {
		cfgMap, ok := cfg.(map[string]any)
		if !ok {
			continue
		}
		for _, field := range environmentSecretFields {
			delete(cfgMap, field)
		}
	}
}

// scrubFlagDocument parses a flag or flag-list payload once and removes
// environment secrets from every flag it contains.
func scrubFlagDocument(data []byte) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("launchdarkly: decode flag response: %w", err)
	}
	if items, ok := doc["items"].([]any); ok {
		for _, item := range items {
			if flag, ok := item.(map[string]any); ok {
				scrubEnvironments(flag)
			}
		}
		return doc, nil
	}
	scrubEnvironments(doc)
	return doc, nil
}

func scrubbedFlagResult(data []byte) (*mcp.ToolResult, error) {
	doc, err := scrubFlagDocument(data)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(doc)
}

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
	return scrubbedFlagResult(data)
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
	return scrubbedFlagResult(data)
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
	doc, err := scrubFlagDocument(data)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(toggleConfirmation(doc, envKey))
}

// toggleConfirmation reduces the full flag document returned by a semantic
// patch to the fields that confirm the toggle took effect in one environment.
func toggleConfirmation(doc map[string]any, envKey string) map[string]any {
	out := map[string]any{
		"key":             doc["key"],
		"environment_key": envKey,
		"status":          "updated",
	}
	envs, _ := doc["environments"].(map[string]any)
	cfg, _ := envs[envKey].(map[string]any)
	for _, field := range []string{"on", "version", "lastModified"} {
		if v, ok := cfg[field]; ok {
			out[field] = v
		}
	}
	return out
}
