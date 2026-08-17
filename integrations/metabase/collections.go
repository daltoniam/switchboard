package metabase

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listCollections(ctx context.Context, m *metabase, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.get(ctx, "/api/collection")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCollection(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "collection_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("collection_id is required"))
	}
	data, err := m.get(ctx, "/api/collection/%s/items", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCollection(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	parentID := r.Int("parent_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}

	body := map[string]any{"name": name}
	if description != "" {
		body["description"] = description
	}
	if parentID != 0 {
		body["parent_id"] = parentID
	}

	data, err := m.post(ctx, "/api/collection", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCollection(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("collection_id")
	name := r.Str("name")
	description := r.Str("description")
	parentID := r.Int("parent_id")
	archived := r.Bool("archived")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("collection_id is required"))
	}

	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if parentID != 0 {
		body["parent_id"] = parentID
	}
	if archived {
		body["archived"] = true
	}

	data, err := m.put(ctx, fmt.Sprintf("/api/collection/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func search(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	models := r.Str("models")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if query == "" {
		return mcp.ErrResult(fmt.Errorf("query is required"))
	}

	path := fmt.Sprintf("/api/search?q=%s", query)
	if models != "" {
		for _, model := range strings.Split(models, ",") {
			path += "&models=" + strings.TrimSpace(model)
		}
	}

	data, err := m.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
