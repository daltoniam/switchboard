package metabase

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDatabases(ctx context.Context, m *metabase, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.get(ctx, "/api/database")
	if err != nil {
		return mcp.ErrResult(err)
	}
	// Metabase wraps the list in {"data": [...]}. Unwrap so compaction specs
	// can target the array items directly.
	return mcp.RawResult(unwrapData(data))
}

// unwrapData extracts the "data" array from a Metabase API envelope.
// Returns the original data unchanged if it doesn't match the envelope pattern.
func unwrapData(data json.RawMessage) json.RawMessage {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &envelope) == nil && len(envelope.Data) > 0 {
		return envelope.Data
	}
	return data
}

func getDatabase(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "database_id")
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("database_id is required"))
	}
	data, err := m.get(ctx, "/api/database/%d?include=tables", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listTables(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "database_id")
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("database_id is required"))
	}
	data, err := m.get(ctx, "/api/database/%d/metadata", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTable(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "table_id")
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	data, err := m.get(ctx, "/api/table/%d", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTableFields(ctx context.Context, m *metabase, args map[string]any) (*mcp.ToolResult, error) {
	id := argInt(args, "table_id")
	if id == 0 {
		return mcp.ErrResult(fmt.Errorf("table_id is required"))
	}
	data, err := m.get(ctx, "/api/table/%d/query_metadata", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
