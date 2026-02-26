package metabase

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func executeQuery(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	dbID := argInt(args, "database_id")
	query := argStr(args, "query")
	if dbID == 0 || query == "" {
		return errResult(fmt.Errorf("database_id and query are required"))
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
		return errResult(err)
	}
	return rawResult(data)
}

func executeCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	cardID := argInt(args, "card_id")
	if cardID == 0 {
		return errResult(fmt.Errorf("card_id is required"))
	}

	path := fmt.Sprintf("/api/card/%d/query", cardID)
	params := argStr(args, "parameters")
	if params != "" {
		var p []any
		if err := json.Unmarshal([]byte(params), &p); err != nil {
			return errResult(fmt.Errorf("invalid parameters JSON: %w", err))
		}
		data, err := m.post(ctx, path, map[string]any{"parameters": p})
		if err != nil {
			return errResult(err)
		}
		return rawResult(data)
	}

	data, err := m.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listCards(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	filter := argStr(args, "filter")
	path := "/api/card"
	if filter != "" {
		path = fmt.Sprintf("/api/card?f=%s", filter)
	}
	data, err := m.get(ctx, "%s", path)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "card_id")
	if id == 0 {
		return errResult(fmt.Errorf("card_id is required"))
	}
	data, err := m.get(ctx, "/api/card/%d", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "name")
	dbID := argInt(args, "database_id")
	query := argStr(args, "query")
	if name == "" || dbID == 0 || query == "" {
		return errResult(fmt.Errorf("name, database_id, and query are required"))
	}

	display := argStr(args, "display")
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

	if desc := argStr(args, "description"); desc != "" {
		body["description"] = desc
	}
	if cid := argInt(args, "collection_id"); cid != 0 {
		body["collection_id"] = cid
	}

	data, err := m.post(ctx, "/api/card", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "card_id")
	if id == 0 {
		return errResult(fmt.Errorf("card_id is required"))
	}

	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if v := argStr(args, "display"); v != "" {
		body["display"] = v
	}
	if argBool(args, "archived") {
		body["archived"] = true
	}

	if q := argStr(args, "query"); q != "" {
		dbID := argInt(args, "database_id")
		if dbID == 0 {
			return errResult(fmt.Errorf("database_id is required when updating query"))
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
		return errResult(err)
	}
	return rawResult(data)
}

func deleteCard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "card_id")
	if id == 0 {
		return errResult(fmt.Errorf("card_id is required"))
	}
	data, err := m.del(ctx, "/api/card/%d", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
