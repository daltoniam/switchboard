package metabase

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDatabases(ctx context.Context, m *metabase, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.get(ctx, "/api/database")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getDatabase(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "database_id")
	if id == 0 {
		return errResult(fmt.Errorf("database_id is required"))
	}
	data, err := m.get(ctx, "/api/database/%d?include=tables", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listTables(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "database_id")
	if id == 0 {
		return errResult(fmt.Errorf("database_id is required"))
	}
	data, err := m.get(ctx, "/api/database/%d/metadata", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getTable(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "table_id")
	if id == 0 {
		return errResult(fmt.Errorf("table_id is required"))
	}
	data, err := m.get(ctx, "/api/table/%d", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getTableFields(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "table_id")
	if id == 0 {
		return errResult(fmt.Errorf("table_id is required"))
	}
	data, err := m.get(ctx, "/api/table/%d/query_metadata", id)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
