package metabase

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func executeQuery(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	dbID := r.Int("database_id")
	query := r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if dbID == 0 || query == "" {
		return mcp.ErrResult(fmt.Errorf("database_id and query are required"))
	}

	body := map[string]any{
		"database": dbID,
		"type":     "native",
		"native": map[string]any{
			"query":         query,
			"template-tags": map[string]any{},
		},
	}

	data, err := m.post(ctx, "/api/dataset", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func executeCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	cardID := r.Int("card_id")
	params := r.Str("parameters")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if cardID == 0 {
		return mcp.ErrResult(fmt.Errorf("card_id is required"))
	}

	path := fmt.Sprintf("/api/card/%d/query", cardID)
	if params != "" {
		var p []any
		if err := json.Unmarshal([]byte(params), &p); err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid parameters JSON: %w", err))
		}
		data, err := m.post(ctx, path, map[string]any{"parameters": p})
		if err != nil {
			return mcp.ErrResult(err)
		}
		return mcp.RawResult(data)
	}

	data, err := m.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCards(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	filter, err := mcp.ArgStr(args, "filter")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := "/api/card"
	if filter != "" {
		path = fmt.Sprintf("/api/card?f=%s", filter)
	}
	data, err := m.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "card_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("card_id is required"))
	}
	data, err := m.get(ctx, "/api/card/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	dbID := r.Int("database_id")
	query := r.Str("query")
	display := r.Str("display")
	desc := r.Str("description")
	cid := r.Int("collection_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" || dbID == 0 || query == "" {
		return mcp.ErrResult(fmt.Errorf("name, database_id, and query are required"))
	}

	if display == "" {
		display = "table"
	}

	body := map[string]any{
		"name":    name,
		"display": display,
		"dataset_query": map[string]any{
			"database": dbID,
			"type":     "native",
			"native": map[string]any{
				"query":         query,
				"template-tags": map[string]any{},
			},
		},
		"visualization_settings": map[string]any{},
	}

	if desc != "" {
		body["description"] = desc
	}
	if cid != 0 {
		body["collection_id"] = cid
	}

	data, err := m.post(ctx, "/api/card", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Int("card_id")
	name := r.Str("name")
	description := r.Str("description")
	display := r.Str("display")
	archived := r.Bool("archived")
	q := r.Str("query")
	dbID := r.Int("database_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("card_id is required"))
	}

	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if display != "" {
		body["display"] = display
	}
	if archived {
		body["archived"] = true
	}

	if q != "" {
		if dbID == 0 {
			return mcp.ErrResult(fmt.Errorf("database_id is required when updating query"))
		}
		body["dataset_query"] = map[string]any{
			"database": dbID,
			"type":     "native",
			"native": map[string]any{
				"query":         q,
				"template-tags": map[string]any{},
			},
		}
	}

	data, err := m.put(ctx, fmt.Sprintf("/api/card/%d", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "card_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("card_id is required"))
	}
	data, err := m.del(ctx, "/api/card/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
