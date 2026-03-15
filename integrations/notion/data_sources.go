package notion

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func createDatabase(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	parent := argMap(args, "parent")
	if parent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}

	parentID, _ := resolveParent(parent)
	if parentID == "" {
		return errResult(fmt.Errorf("parent must contain page_id or database_id"))
	}

	blockID := newBlockID()
	collectionID := newBlockID()
	viewID := newBlockID()
	now := currentTimeMillis()

	// Build schema from properties arg or default to title-only
	schema := map[string]any{
		"title": map[string]any{"name": "Name", "type": "title"},
	}
	if props := argMap(args, "properties"); props != nil {
		schema = props
	}

	// Collection name
	title := argStr(args, "title")
	var nameValue any
	if title != "" {
		nameValue = []any{[]any{title}}
	}

	// Block: collection_view_page
	blockData := map[string]any{
		"id":                   blockID,
		"type":                 "collection_view_page",
		"collection_id":        collectionID,
		"view_ids":             []string{viewID},
		"parent_id":            parentID,
		"parent_table":         "block",
		"space_id":             n.spaceID,
		"created_by_id":        n.userID,
		"created_by_table":     "notion_user",
		"last_edited_by_id":    n.userID,
		"last_edited_by_table": "notion_user",
		"alive":                true,
		"created_time":         now,
		"last_edited_time":     now,
	}

	// Collection
	collectionData := map[string]any{
		"id":           collectionID,
		"schema":       schema,
		"parent_id":    blockID,
		"parent_table": "block",
		"space_id":     n.spaceID,
		"alive":        true,
	}
	if nameValue != nil {
		collectionData["name"] = nameValue
	}

	// Collection view — gotcha: parent_table must be "block"
	viewData := map[string]any{
		"id":           viewID,
		"type":         "table",
		"parent_id":    blockID,
		"parent_table": "block",
		"space_id":     n.spaceID,
		"alive":        true,
		"name":         "Default view",
	}

	ops := []op{
		buildSetOp("block", blockID, []string{}, blockData),
		buildSetOp("collection", collectionID, []string{}, collectionData),
		buildSetOp("collection_view", viewID, []string{}, viewData),
		buildListAfterOp("block", parentID, []string{"content"}, map[string]any{
			"id": blockID,
		}),
		buildUpdateOp("block", blockID, []string{}, map[string]any{
			"last_edited_time": now,
		}),
	}

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{
		"id":            blockID,
		"collection_id": collectionID,
		"view_id":       viewID,
		"url":           notionPageURL(blockID),
	})
}

func retrieveDataSource(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	// loadCachedPageChunkV2 includes both block and collection records in recordMap.
	// Single call resolves the block → extracts collection — no getRecordValues needed.
	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": dataSourceID},
		"limit":           1,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	})
	if err != nil {
		return errResult(err)
	}

	// Find the block to get its collection_id
	block, err := extractRecord(data, "block", dataSourceID)
	if err != nil {
		return errResult(err)
	}
	collectionID, _ := block["collection_id"].(string)
	if collectionID == "" {
		return errResult(fmt.Errorf("block %q is not a database (no collection_id)", dataSourceID))
	}

	// Collection is included in the same recordMap response
	return recordMapResult(data, "collection", collectionID)
}

func updateDataSource(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	// Resolve: data_source_id may be a block ID from search
	collectionID := dataSourceID
	resolved, _, err := resolveDataSource(ctx, n, dataSourceID)
	if err == nil {
		collectionID = resolved
	}

	var ops []op

	if title := argStr(args, "title"); title != "" {
		ops = append(ops, buildSetOp("collection", collectionID, []string{"name"}, []any{[]any{title}}))
	}

	if props := argMap(args, "properties"); props != nil {
		ops = append(ops, buildSetOp("collection", collectionID, []string{"schema"}, props))
	}

	if len(ops) == 0 {
		return errResult(fmt.Errorf("no updates specified: provide title or properties"))
	}

	_, err = submitTransaction(ctx, n, ops)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"id": collectionID, "status": "updated"})
}

func queryDataSource(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	// Step 1: Resolve data_source_id (may be a block ID from search) to collection + view IDs
	collectionID, viewID, err := resolveDataSource(ctx, n, dataSourceID)
	if err != nil {
		return errResult(err)
	}

	// Step 2: Build queryCollection request — source + reducer format
	// Pattern confirmed via Playwright capture and shell script testing.
	limit := 50
	if v := argInt(args, "page_size"); v > 0 {
		if v > 100 {
			v = 100
		}
		limit = v
	}

	sortVal := []any{}
	if v, ok := args["sorts"]; ok && v != nil {
		sortVal, _ = v.([]any)
	}

	loader := map[string]any{
		"reducers": map[string]any{
			"collection_group_results": map[string]any{
				"type":  "results",
				"limit": limit,
			},
		},
		"sort":         sortVal,
		"searchQuery":  "",
		"userTimeZone": "America/Los_Angeles",
	}

	if v := argMap(args, "filter"); v != nil {
		loader["filter"] = v
	}

	body := map[string]any{
		"source": map[string]any{
			"type":    "collection",
			"id":      collectionID,
			"spaceId": n.spaceID,
		},
		"collectionView": map[string]any{
			"id":      viewID,
			"spaceId": n.spaceID,
		},
		"loader": loader,
	}

	data, err := n.doRequest(ctx, "/api/v3/queryCollection", body)
	if err != nil {
		return errResult(err)
	}

	// Step 3: Parse response — reducerResults format with double-wrapped blocks.
	// recordMap contains a top-level "__version__" (number) alongside table maps,
	// so we parse it as map[string]any to tolerate heterogeneous values.
	var resp map[string]any
	if err := unmarshalJSON(data, &resp); err != nil {
		return errResult(err)
	}

	resultObj, _ := resp["result"].(map[string]any)
	reducerResults, _ := resultObj["reducerResults"].(map[string]any)
	groupResults, _ := reducerResults["collection_group_results"].(map[string]any)
	blockIDs := toStringSlice(groupResults["blockIds"])
	hasMore, _ := groupResults["hasMore"].(bool)

	allBlockIDs := toStringSlice(resp["allBlockIds"])
	total := len(allBlockIDs)
	if total == 0 {
		if sh, ok := resultObj["sizeHint"].(float64); ok {
			total = int(sh)
		}
	}

	// Extract rows in blockIds order, handling double-wrapped block[id].value.value
	recordMap, _ := resp["recordMap"].(map[string]any)
	blockTable, _ := recordMap["block"].(map[string]any)
	var results []any
	for _, blockID := range blockIDs {
		entry, ok := blockTable[blockID].(map[string]any)
		if !ok {
			continue
		}
		outerMap, ok := entry["value"].(map[string]any)
		if !ok {
			continue
		}
		// queryCollection double-wraps: block[id].value is {value: {actual data}}
		if inner, ok := outerMap["value"]; ok {
			results = append(results, inner)
			continue
		}
		results = append(results, outerMap)
	}

	// Extract collection schema from recordMap — maps opaque property keys to names.
	// Same double-wrap as blocks: collection[id].value.value.schema
	schema := extractCollectionSchema(recordMap, collectionID)

	out := map[string]any{
		"results":  results,
		"total":    total,
		"has_more": hasMore,
	}
	if schema != nil {
		out["schema"] = schema
	}
	return jsonResult(out)
}

// resolveDataSource resolves a data_source_id (which may be a collection_view_page
// block ID from search results) to the actual collection ID and first view ID.
// Uses loadCachedPageChunkV2 which is reliable even when getRecordValues is shard-isolated.
func resolveDataSource(ctx context.Context, n *notion, id string) (collectionID, viewID string, err error) {
	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": id},
		"limit":           1,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	})
	if err != nil {
		return "", "", fmt.Errorf("resolve data source: %w", err)
	}

	blocks, err := extractAllRecords(data, "block")
	if err != nil {
		return "", "", err
	}

	for _, block := range blocks {
		bid, _ := block["id"].(string)
		if bid != id {
			continue
		}
		colID, _ := block["collection_id"].(string)
		if colID == "" {
			return "", "", fmt.Errorf("block %q is not a database (no collection_id)", id)
		}
		viewIDs := toStringSlice(block["view_ids"])
		if len(viewIDs) == 0 {
			return "", "", fmt.Errorf("database %q has no views", id)
		}
		return colID, viewIDs[0], nil
	}
	return "", "", fmt.Errorf("data source %q not found", id)
}

// extractCollectionSchema extracts the schema field from a collection record in
// a queryCollection recordMap. Handles double-wrapped collection[id].value.value.schema.
// Returns nil if collection is missing or has no schema.
func extractCollectionSchema(recordMap map[string]any, collectionID string) map[string]any {
	collTable, _ := recordMap["collection"].(map[string]any)
	entry, _ := collTable[collectionID].(map[string]any)
	outer, _ := entry["value"].(map[string]any)
	// Double-wrapped like blocks: value.value.schema
	if inner, ok := outer["value"].(map[string]any); ok {
		schema, _ := inner["schema"].(map[string]any)
		return schema
	}
	// Single-wrapped fallback: value.schema
	schema, _ := outer["schema"].(map[string]any)
	return schema
}

func retrieveDatabase(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	databaseID := argStr(args, "database_id")
	if databaseID == "" {
		return errResult(fmt.Errorf("database_id is required"))
	}

	// Same pattern as retrieveDataSource — single loadCachedPageChunkV2 call
	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": databaseID},
		"limit":           1,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	})
	if err != nil {
		return errResult(err)
	}

	block, err := extractRecord(data, "block", databaseID)
	if err != nil {
		return errResult(err)
	}
	collectionID, _ := block["collection_id"].(string)
	if collectionID == "" {
		return errResult(fmt.Errorf("block %q is not a database (no collection_id)", databaseID))
	}

	return recordMapResult(data, "collection", collectionID)
}

func listDataSourceTemplates(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	dataSourceID := argStr(args, "data_source_id")
	if dataSourceID == "" {
		return errResult(fmt.Errorf("data_source_id is required"))
	}

	// Load the page chunk — includes block + collection records in recordMap
	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": dataSourceID},
		"limit":           100,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	})
	if err != nil {
		return errResult(err)
	}

	// Find the block to get collection_id
	block, err := extractRecord(data, "block", dataSourceID)
	if err != nil {
		return errResult(err)
	}
	collectionID, _ := block["collection_id"].(string)
	if collectionID == "" {
		return errResult(fmt.Errorf("block %q is not a database (no collection_id)", dataSourceID))
	}

	// Get collection to find template_pages
	collection, err := extractRecord(data, "collection", collectionID)
	if err != nil {
		return errResult(fmt.Errorf("collection %q not found", collectionID))
	}

	templateIDs := toStringSlice(collection["template_pages"])
	if len(templateIDs) == 0 {
		return jsonResult(map[string]any{"results": []any{}})
	}

	// Fetch each template block via loadCachedPageChunkV2
	var templates []map[string]any
	for _, tmplID := range templateIDs {
		tmplResult, err := loadBlock(ctx, n, tmplID)
		if err != nil {
			continue
		}
		if tmplResult.IsError {
			continue
		}
		var tmpl map[string]any
		if err := unmarshalJSON([]byte(tmplResult.Data), &tmpl); err != nil {
			continue
		}
		templates = append(templates, tmpl)
	}

	return jsonResult(map[string]any{"results": templates})
}
