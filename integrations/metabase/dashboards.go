package metabase

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDashboards(ctx context.Context, m *metabase, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.get(ctx, "/api/dashboard")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "dashboard_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("dashboard_id is required"))
	}
	data, err := m.get(ctx, "/api/dashboard/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	description := r.Str("description")
	cid := r.Int("collection_id")
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
	if cid != 0 {
		body["collection_id"] = cid
	}

	data, err := m.post(ctx, "/api/dashboard", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Int("dashboard_id")
	name := r.Str("name")
	description := r.Str("description")
	archived := r.Bool("archived")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("dashboard_id is required"))
	}

	body := map[string]any{}
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}
	if archived {
		body["archived"] = true
	}

	data, err := m.put(ctx, fmt.Sprintf("/api/dashboard/%d", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgInt(args, "dashboard_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("dashboard_id is required"))
	}
	data, err := m.del(ctx, "/api/dashboard/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func addCardToDashboard(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	dashID := r.Int("dashboard_id")
	cardID := r.Int("card_id")
	sizeX := r.Int("size_x")
	sizeY := r.Int("size_y")
	row := r.Int("row")
	col := r.Int("col")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if dashID == 0 || cardID == 0 {
		return mcp.ErrResult(fmt.Errorf("dashboard_id and card_id are required"))
	}

	if sizeX == 0 {
		sizeX = 6
	}
	if sizeY == 0 {
		sizeY = 4
	}

	body := map[string]any{
		"cardId": cardID,
		"size_x": sizeX,
		"size_y": sizeY,
		"row":    row,
		"col":    col,
	}

	data, err := m.put(ctx, fmt.Sprintf("/api/dashboard/%d", dashID), map[string]any{
		"dashcards": []any{body},
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
