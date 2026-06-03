package notion

import (
	"context"
	"errors"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Databases & Data Sources (v1) ---
//
// Notion's 2025-09-03 API split each "database" into a container (which
// owns metadata, title, parent, multiple views) and one or more "data
// sources" (each owns a schema and rows). Older callers using
// `database_id` continue to work — Notion routes them to the database's
// first data source automatically.
//
// Tool-name mapping:
//   notion_create_database            → POST   /v1/databases
//   notion_retrieve_data_source       → GET    /v1/data_sources/{id}    (falls back to /v1/databases/{id})
//   notion_update_data_source         → PATCH  /v1/data_sources/{id}    (falls back to /v1/databases/{id})
//   notion_query_data_source          → POST   /v1/data_sources/{id}/query  (falls back to /v1/databases/{id}/query)
//   notion_list_data_source_templates → not directly supported; emulated via search
//   notion_retrieve_database          → GET    /v1/databases/{id}

func v1CreateDatabase(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parent := r.Map("parent")
	properties := r.Map("properties")
	title := r.Str("title")
	isInline := r.Bool("is_inline")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if parent == nil {
		return mcp.ErrResult(fmt.Errorf("parent is required"))
	}

	parentObj, err := v1BuildParentObject(parent)
	if err != nil {
		return mcp.ErrResult(err)
	}

	schema := properties
	if schema == nil {
		schema = map[string]any{
			"Name": map[string]any{"title": map[string]any{}},
		}
	}

	body := map[string]any{
		"parent":     parentObj,
		"properties": schema,
		"is_inline":  isInline,
	}
	if title != "" {
		body["title"] = []any{
			map[string]any{
				"type": "text",
				"text": map[string]any{"content": title},
			},
		}
	}

	data, err := n.post(ctx, "/databases", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1RetrieveDataSource(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "data_source_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("data_source_id is required"))
	}
	return v1FetchDatabaseOrDataSource(ctx, n, id)
}

func v1RetrieveDatabase(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "database_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("database_id is required"))
	}
	return v1FetchDatabaseOrDataSource(ctx, n, id)
}

func v1UpdateDataSource(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("data_source_id")
	title := r.Str("title")
	properties := r.Map("properties")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("data_source_id is required"))
	}

	body := map[string]any{}
	if title != "" {
		body["title"] = []any{
			map[string]any{
				"type": "text",
				"text": map[string]any{"content": title},
			},
		}
	}
	if properties != nil {
		body["properties"] = properties
	}
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("at least one of title or properties must be set"))
	}

	// Prefer data_sources; fall back to databases for older IDs.
	// Only fall back on non-retryable failures — if Notion rate-limits us
	// (429) or 5xx's, doRequest returns a *mcp.RetryableError that the
	// runtime knows how to back off on. Swallowing that and immediately
	// firing a second request would double the load and lose the signal.
	data, err := n.patch(ctx, "/data_sources/"+id, body)
	if err != nil {
		var re *mcp.RetryableError
		if errors.As(err, &re) {
			return mcp.ErrResult(err)
		}
		data, err = n.patch(ctx, "/databases/"+id, body)
		if err != nil {
			return mcp.ErrResult(err)
		}
	}
	return mcp.RawResult(data)
}

func v1QueryDataSource(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("data_source_id")
	filter := r.Map("filter")
	startCursor := r.Str("start_cursor")
	pageSize := r.Int("page_size")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("data_source_id is required"))
	}

	body := map[string]any{}
	if filter != nil {
		body["filter"] = filter
	}
	if sorts, ok := args["sorts"].([]any); ok && len(sorts) > 0 {
		body["sorts"] = sorts
	}
	if startCursor != "" {
		body["start_cursor"] = startCursor
	}
	if pageSize > 0 {
		if pageSize > 100 {
			pageSize = 100
		}
		body["page_size"] = pageSize
	}

	data, err := n.post(ctx, "/data_sources/"+id+"/query", body)
	if err != nil {
		var re *mcp.RetryableError
		if errors.As(err, &re) {
			return mcp.ErrResult(err)
		}
		// Older callers may pass a database_id under data_source_id.
		data, err = n.post(ctx, "/databases/"+id+"/query", body)
		if err != nil {
			return mcp.ErrResult(err)
		}
	}
	return mcp.RawResult(data)
}

// v1ListDataSourceTemplates is a best-effort port. Notion's public v1 API
// does NOT expose a "list database templates" endpoint — the templates
// model is partially internal. We return an empty result with an
// explanatory note rather than fail, so workflows that branch on
// "templates exist?" keep working.
func v1ListDataSourceTemplates(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "data_source_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("data_source_id is required"))
	}
	_ = ctx
	_ = n
	return mcp.JSONResult(map[string]any{
		"results": []any{},
		"note":    "Notion v1 API does not expose database templates; this tool returns an empty list on the OAuth backend.",
	})
}

// v1FetchDatabaseOrDataSource tries /data_sources/{id} first (the
// 2025-09-03 endpoint), falls back to /databases/{id} for IDs that pre-date
// the split or for callers that pass the container ID interchangeably.
func v1FetchDatabaseOrDataSource(ctx context.Context, n *notionV1, id string) (*mcp.ToolResult, error) {
	data, err := n.get(ctx, "/data_sources/%s", id)
	if err == nil {
		return mcp.RawResult(data)
	}
	// Don't swallow rate-limit / 5xx — those need to bubble up as
	// retryable so the runtime backs off.
	var re *mcp.RetryableError
	if errors.As(err, &re) {
		return mcp.ErrResult(err)
	}
	data, err2 := n.get(ctx, "/databases/%s", id)
	if err2 != nil {
		// Surface the original error since it's the more likely path.
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
