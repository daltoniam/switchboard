package notion

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// buildParentLinkOps returns the ops to link a new page to its parent.
// Collection parents use setParent; block parents use listAfter on content.
func buildParentLinkOps(n *notion, pageID, parentID, parentTable string) []op {
	if parentTable == "collection" {
		return []op{
			buildSetParentOp(n.spaceID, pageID, parentID, parentTable),
		}
	}
	return []op{
		buildListAfterOp("block", parentID, []string{"content"}, map[string]any{
			"id": pageID,
		}),
	}
}

// buildParentUnlinkOps returns the ops to unlink a page from its current parent.
// Collection parents have no content list, so no listRemove is needed.
func buildParentUnlinkOps(pageID, parentID, parentTable string) []op {
	if parentTable == "collection" {
		return nil
	}
	return []op{
		buildListRemoveOp("block", parentID, []string{"content"}, map[string]any{
			"id": pageID,
		}),
	}
}

func createPage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parent := r.Map("parent")
	properties := r.Map("properties")
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if parent == nil {
		return mcp.ErrResult(fmt.Errorf("parent is required"))
	}

	parentID, parentTable, err := resolveParent(parent)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if parentID == "" {
		return mcp.ErrResult(fmt.Errorf("parent must contain page_id or database_id"))
	}

	pageID := newBlockID()
	now := currentTimeMillis()

	blockData := map[string]any{
		"id":                   pageID,
		"type":                 "page",
		"parent_id":            parentID,
		"parent_table":         parentTable,
		"space_id":             n.spaceID,
		"created_by_id":        n.userID,
		"created_by_table":     "notion_user",
		"last_edited_by_id":    n.userID,
		"last_edited_by_table": "notion_user",
		"alive":                true,
		"created_time":         now,
		"last_edited_time":     now,
	}

	if properties != nil {
		blockData["properties"] = properties
	}
	if title != "" {
		if blockData["properties"] == nil {
			blockData["properties"] = map[string]any{}
		}
		blockData["properties"].(map[string]any)["title"] = []any{[]any{title}}
	}

	ops := []op{
		buildSetOp("block", pageID, []string{}, blockData),
		buildUpdateOp("block", pageID, []string{}, map[string]any{
			"last_edited_time": now,
		}),
	}
	ops = append(ops, buildParentLinkOps(n, pageID, parentID, parentTable)...)

	_, err = submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": pageID, "url": notionPageURL(pageID)})
}

func retrievePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := mcp.ArgStr(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}
	return loadBlock(ctx, n, pageID)
}

func updatePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	props := r.Map("properties")
	archived := r.Bool("archived")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}

	var ops []op

	if props != nil {
		ops = append(ops, buildSetOp("block", pageID, []string{"properties"}, props))
	}

	if archived {
		ops = append(ops, buildSetOp("block", pageID, []string{"alive"}, false))
	}

	now := currentTimeMillis()
	ops = append(ops, buildUpdateOp("block", pageID, []string{}, map[string]any{
		"last_edited_time": now,
	}))

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": pageID, "status": "updated"})
}

func movePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	newParent := r.Map("parent")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}
	if newParent == nil {
		return mcp.ErrResult(fmt.Errorf("parent is required"))
	}

	newParentID, newParentTable, err := resolveParent(newParent)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if newParentID == "" {
		return mcp.ErrResult(fmt.Errorf("parent must contain page_id or database_id"))
	}

	// Fetch current page to get old parent_id
	result, err := loadBlock(ctx, n, pageID)
	if err != nil {
		return nil, err
	}
	if result.IsError {
		return result, nil
	}

	var page map[string]any
	if err := unmarshalJSON([]byte(result.Data), &page); err != nil {
		return mcp.ErrResult(err)
	}
	oldParentID, _ := page["parent_id"].(string)
	oldParentTable, _ := page["parent_table"].(string)
	if oldParentTable == "" {
		oldParentTable = "block"
	}

	now := currentTimeMillis()
	var ops []op

	// Remove from old parent (no-op for collection parents)
	ops = append(ops, buildParentUnlinkOps(pageID, oldParentID, oldParentTable)...)

	// Update parent_id and parent_table
	ops = append(ops,
		buildSetOp("block", pageID, []string{"parent_id"}, newParentID),
		buildSetOp("block", pageID, []string{"parent_table"}, newParentTable),
	)

	// Link to new parent
	ops = append(ops, buildParentLinkOps(n, pageID, newParentID, newParentTable)...)

	ops = append(ops, buildUpdateOp("block", pageID, []string{}, map[string]any{
		"last_edited_time": now,
	}))

	_, err = submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": pageID, "status": "moved"})
}

func retrievePageProperty(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	propertyID := r.Str("property_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}
	if propertyID == "" {
		return mcp.ErrResult(fmt.Errorf("property_id is required"))
	}

	// Fetch the page and extract the specific property.
	result, err := loadBlock(ctx, n, pageID)
	if err != nil {
		return nil, err
	}
	if result.IsError {
		return result, nil
	}

	var page map[string]any
	if err := unmarshalJSON([]byte(result.Data), &page); err != nil {
		return mcp.ErrResult(err)
	}

	// Look in properties map for the requested property
	props, _ := page["properties"].(map[string]any)
	if props == nil {
		return mcp.JSONResult(map[string]any{"property_id": propertyID, "value": nil})
	}

	propVal, exists := props[propertyID]
	if !exists {
		return mcp.JSONResult(map[string]any{"property_id": propertyID, "value": nil})
	}

	return mcp.JSONResult(map[string]any{"property_id": propertyID, "value": propVal})
}

func getPageContent(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}

	if limit <= 0 {
		limit = 100
	}

	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": pageID},
		"limit":           limit,
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

	// Index blocks by ID for ordered retrieval.
	blockByID := make(map[string]map[string]any, len(blocks))
	var page map[string]any
	for _, block := range blocks {
		id, _ := block["id"].(string)
		if id == pageID {
			page = block
			continue
		}
		blockByID[id] = block
	}
	if page == nil {
		return mcp.ErrResult(fmt.Errorf("page %q not found in response", pageID))
	}

	// Order children by the page's content array (defines display order).
	contentIDs := toStringSlice(page["content"])
	children := make([]map[string]any, 0, len(blockByID))
	for _, id := range contentIDs {
		if block, ok := blockByID[id]; ok {
			children = append(children, block)
			delete(blockByID, id)
		}
	}
	// Append any remaining blocks not in content array (nested children).
	for _, block := range blockByID {
		children = append(children, block)
	}

	return mcp.JSONResult(map[string]any{
		"page":   page,
		"blocks": children,
	})
}

func createPageWithContent(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parent := r.Map("parent")
	properties := r.Map("properties")
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if parent == nil {
		return mcp.ErrResult(fmt.Errorf("parent is required"))
	}

	children, ok := args["children"].([]any)
	if !ok || len(children) == 0 {
		return mcp.ErrResult(fmt.Errorf("children is required"))
	}

	parentID, parentTable, err := resolveParent(parent)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if parentID == "" {
		return mcp.ErrResult(fmt.Errorf("parent must contain page_id or database_id"))
	}

	pageID := newBlockID()
	now := currentTimeMillis()

	blockData := map[string]any{
		"id":                   pageID,
		"type":                 "page",
		"parent_id":            parentID,
		"parent_table":         parentTable,
		"space_id":             n.spaceID,
		"created_by_id":        n.userID,
		"created_by_table":     "notion_user",
		"last_edited_by_id":    n.userID,
		"last_edited_by_table": "notion_user",
		"alive":                true,
		"created_time":         now,
		"last_edited_time":     now,
	}

	if properties != nil {
		blockData["properties"] = properties
	}
	if title != "" {
		if blockData["properties"] == nil {
			blockData["properties"] = map[string]any{}
		}
		blockData["properties"].(map[string]any)["title"] = []any{[]any{title}}
	}

	ops := []op{
		// Create page block
		buildSetOp("block", pageID, []string{}, blockData),
		buildUpdateOp("block", pageID, []string{}, map[string]any{
			"last_edited_time": now,
		}),
	}
	// Link page to parent
	ops = append(ops, buildParentLinkOps(n, pageID, parentID, parentTable)...)

	// Create child blocks
	for _, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		childOps := buildChildBlockOps(n, pageID, childMap, now)
		ops = append(ops, childOps...)
	}

	_, err = submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": pageID, "url": notionPageURL(pageID)})
}
