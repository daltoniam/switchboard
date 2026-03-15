package notion

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func retrieveBlock(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}
	return loadBlock(ctx, n, blockID)
}

func updateBlock(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}

	var ops []op

	if content := argMap(args, "type_content"); content != nil {
		if props, ok := content["properties"].(map[string]any); ok {
			ops = append(ops, buildSetOp("block", blockID, []string{"properties"}, props))
		}
		if format, ok := content["format"].(map[string]any); ok {
			ops = append(ops, buildSetOp("block", blockID, []string{"format"}, format))
		}
	}

	if argBool(args, "archived") {
		ops = append(ops, buildSetOp("block", blockID, []string{"alive"}, false))
	}

	now := currentTimeMillis()
	ops = append(ops, buildUpdateOp("block", blockID, []string{}, map[string]any{
		"last_edited_time": now,
	}))

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": blockID, "status": "updated"})
}

func deleteBlock(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}

	// Fetch block to get parent info for listRemove
	result, err := loadBlock(ctx, n, blockID)
	if err != nil {
		return nil, err
	}
	if result.IsError {
		return result, nil
	}

	var block map[string]any
	if err := unmarshalJSON([]byte(result.Data), &block); err != nil {
		return mcp.ErrResult(err)
	}
	parentID, _ := block["parent_id"].(string)
	parentTable, _ := block["parent_table"].(string)
	if parentTable == "" {
		parentTable = "block"
	}

	now := currentTimeMillis()
	ops := []op{
		buildSetOp("block", blockID, []string{"alive"}, false),
		buildListRemoveOp(parentTable, parentID, []string{"content"}, map[string]any{
			"id": blockID,
		}),
		buildUpdateOp("block", blockID, []string{}, map[string]any{
			"last_edited_time": now,
		}),
	}

	_, err = submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": blockID, "status": "deleted"})
}

func getBlockChildren(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}

	// Single call: loadCachedPageChunkV2 returns the target block + all descendants.
	// This avoids getRecordValues which is prone to shard isolation 500s.
	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": blockID},
		"limit":           100,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	})
	if err != nil {
		return mcp.ErrResult(err)
	}

	blocks, err := extractAllRecords(data, "block")
	if err != nil {
		return mcp.ErrResult(err)
	}

	// Find the target block to get its content array
	var contentIDs []string
	for _, block := range blocks {
		id, _ := block["id"].(string)
		if id == blockID {
			contentIDs = toStringSlice(block["content"])
			break
		}
	}
	if len(contentIDs) == 0 {
		return mcp.JSONResult(map[string]any{"results": []any{}})
	}

	// Filter to direct children only
	childSet := make(map[string]bool, len(contentIDs))
	for _, id := range contentIDs {
		childSet[id] = true
	}

	var children []map[string]any
	for _, block := range blocks {
		id, _ := block["id"].(string)
		if !childSet[id] {
			continue
		}
		children = append(children, block)
	}

	return mcp.JSONResult(map[string]any{"results": children})
}

func appendBlockChildren(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}

	children, ok := args["children"].([]any)
	if !ok || len(children) == 0 {
		return mcp.ErrResult(fmt.Errorf("children is required"))
	}

	now := currentTimeMillis()
	var ops []op
	var childIDs []string

	for _, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		childOps := buildChildBlockOps(n, blockID, childMap, now)
		ops = append(ops, childOps...)
		// Extract child ID from the first (set) op
		if len(childOps) > 0 {
			childIDs = append(childIDs, childOps[0].ID)
		}
	}

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"block_ids": childIDs})
}

// loadBlock fetches a single block via loadCachedPageChunkV2.
// This replaces getRecordValue for blocks, which fails due to shard isolation.
// Pattern discovered via Playwright capture of Notion web client.
func loadBlock(ctx context.Context, n *notion, blockID string) (*mcp.ToolResult, error) {
	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": blockID},
		"limit":           1,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return recordMapResult(data, "block", blockID)
}

// toStringSlice converts an any slice (from JSON) to []string.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
