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
		return errResult(err)
	}
	return rawResult(data)
}

func getCollection(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "collection_id")
	if id == "" {
		return errResult(fmt.Errorf("collection_id is required"))
	}
	data, err := m.get(ctx, "/api/collection/%s/items", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createCollection(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "name")
	if name == "" {
		return errResult(fmt.Errorf("name is required"))
	}

	body := map[string]any{"name": name}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if pid := argInt(args, "parent_id"); pid != 0 {
		body["parent_id"] = pid
	}

	data, err := m.post(ctx, "/api/collection", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateCollection(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "collection_id")
	if id == "" {
		return errResult(fmt.Errorf("collection_id is required"))
	}

	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if pid := argInt(args, "parent_id"); pid != 0 {
		body["parent_id"] = pid
	}
	if argBool(args, "archived") {
		body["archived"] = true
	}

	data, err := m.put(ctx, fmt.Sprintf("/api/collection/%s", id), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func search(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	query := argStr(args, "query")
	if query == "" {
		return errResult(fmt.Errorf("query is required"))
	}

	path := fmt.Sprintf("/api/search?q=%s", query)
	if models := argStr(args, "models"); models != "" {
		for _, model := range strings.Split(models, ",") {
			path += "&models=" + strings.TrimSpace(model)
		}
	}

	data, err := m.get(ctx, "%s", path)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
