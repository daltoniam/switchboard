package notion

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listUsers(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	// getSpaces returns: { "<user_id>": { "notion_user": { "<uid>": { "value": {...} } }, "space": {...} } }
	data, err := n.doRequest(ctx, "/api/v3/getSpaces", map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}

	// Parse the user-keyed wrapper to find notion_user tables
	var top map[string]json.RawMessage
	if err := unmarshalJSON(data, &top); err != nil {
		return mcp.ErrResult(err)
	}

	var allUsers []map[string]any
	for _, rawTables := range top {
		users, err := extractAllRecords(rawTables, "notion_user")
		if err != nil {
			continue
		}
		allUsers = append(allUsers, users...)
	}

	return mcp.JSONResult(map[string]any{"results": allUsers})
}

// syncRecordValue fetches a single record via syncRecordValuesMain.
// This replaces getRecordValue for user-scoped records (notion_user table),
// which fails due to shard isolation when using getRecordValues.
// Uses the pointer format discovered via Playwright capture of Notion web client.
func syncRecordValue(ctx context.Context, n *notion, table, id string) (*mcp.ToolResult, error) {
	body := map[string]any{
		"requests": []map[string]any{{
			"pointer": map[string]any{
				"table": table,
				"id":    id,
			},
			"version": -1,
		}},
	}
	data, err := n.doRequest(ctx, "/api/v3/syncRecordValuesMain", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return recordMapResult(data, table, id)
}

func retrieveUser(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	userID, err := mcp.ArgStr(args, "user_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if userID == "" {
		return mcp.ErrResult(fmt.Errorf("user_id is required"))
	}
	return syncRecordValue(ctx, n, "notion_user", userID)
}

func getSelf(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	return syncRecordValue(ctx, n, "notion_user", n.userID)
}
