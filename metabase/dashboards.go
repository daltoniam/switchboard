package metabase

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDashboards(ctx context.Context, m *metabase, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.get(ctx, "/api/dashboard")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "dashboard_id")
	if id == 0 {
		return errResult(fmt.Errorf("dashboard_id is required"))
	}
	data, err := m.get(ctx, "/api/dashboard/%d", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	name := argStr(args, "name")
	if name == "" {
		return errResult(fmt.Errorf("name is required"))
	}

	body := map[string]any{"name": name}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if cid := argInt(args, "collection_id"); cid != 0 {
		body["collection_id"] = cid
	}

	data, err := m.post(ctx, "/api/dashboard", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "dashboard_id")
	if id == 0 {
		return errResult(fmt.Errorf("dashboard_id is required"))
	}

	body := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "description"); v != "" {
		body["description"] = v
	}
	if argBool(args, "archived") {
		body["archived"] = true
	}

	data, err := m.put(ctx, fmt.Sprintf("/api/dashboard/%d", id), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "dashboard_id")
	if id == 0 {
		return errResult(fmt.Errorf("dashboard_id is required"))
	}
	data, err := m.del(ctx, "/api/dashboard/%d", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func addCardToDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	dashID := argInt(args, "dashboard_id")
	cardID := argInt(args, "card_id")
	if dashID == 0 || cardID == 0 {
		return errResult(fmt.Errorf("dashboard_id and card_id are required"))
	}

	sizeX := argInt(args, "size_x")
	if sizeX == 0 {
		sizeX = 6
	}
	sizeY := argInt(args, "size_y")
	if sizeY == 0 {
		sizeY = 4
	}

	body := map[string]any{
		"cardId": cardID,
		"size_x": sizeX,
		"size_y": sizeY,
		"row":    argInt(args, "row"),
		"col":    argInt(args, "col"),
	}

	data, err := m.put(ctx, fmt.Sprintf("/api/dashboard/%d", dashID), map[string]any{
		"dashcards": []any{body},
	})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
