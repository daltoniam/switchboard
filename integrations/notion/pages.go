package notion

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func createPage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	parent := argMap(args, "parent")
	if parent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}

	parentID, parentTable := resolveParent(parent)
	if parentID == "" {
		return errResult(fmt.Errorf("parent must contain page_id or database_id"))
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

	if props := argMap(args, "properties"); props != nil {
		blockData["properties"] = props
	}
	if title := argStr(args, "title"); title != "" {
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
		buildListAfterOp("block", parentID, []string{"content"}, map[string]any{
			"id": pageID,
		}),
	}

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"id": pageID, "url": notionPageURL(pageID)})
}

func retrievePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}
	return loadBlock(ctx, n, pageID)
}

func updatePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}

	var ops []op

	if props := argMap(args, "properties"); props != nil {
		ops = append(ops, buildSetOp("block", pageID, []string{"properties"}, props))
	}

	if argBool(args, "archived") {
		ops = append(ops, buildSetOp("block", pageID, []string{"alive"}, false))
	}

	now := currentTimeMillis()
	ops = append(ops, buildUpdateOp("block", pageID, []string{}, map[string]any{
		"last_edited_time": now,
	}))

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"id": pageID, "status": "updated"})
}

func movePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}
	newParent := argMap(args, "parent")
	if newParent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}

	newParentID, newParentTable := resolveParent(newParent)
	if newParentID == "" {
		return errResult(fmt.Errorf("parent must contain page_id or database_id"))
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
		return errResult(err)
	}
	oldParentID, _ := page["parent_id"].(string)

	now := currentTimeMillis()
	ops := []op{
		// Remove from old parent
		buildListRemoveOp("block", oldParentID, []string{"content"}, map[string]any{
			"id": pageID,
		}),
		// Update parent_id and parent_table
		buildSetOp("block", pageID, []string{"parent_id"}, newParentID),
		buildSetOp("block", pageID, []string{"parent_table"}, newParentTable),
		// Add to new parent
		buildListAfterOp("block", newParentID, []string{"content"}, map[string]any{
			"id": pageID,
		}),
		buildUpdateOp("block", pageID, []string{}, map[string]any{
			"last_edited_time": now,
		}),
	}

	_, err = submitTransaction(ctx, n, ops)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"id": pageID, "status": "moved"})
}

func retrievePageProperty(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}
	propertyID := argStr(args, "property_id")
	if propertyID == "" {
		return errResult(fmt.Errorf("property_id is required"))
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
		return errResult(err)
	}

	// Look in properties map for the requested property
	props, _ := page["properties"].(map[string]any)
	if props == nil {
		return jsonResult(map[string]any{"property_id": propertyID, "value": nil})
	}

	propVal, exists := props[propertyID]
	if !exists {
		return jsonResult(map[string]any{"property_id": propertyID, "value": nil})
	}

	return jsonResult(map[string]any{"property_id": propertyID, "value": propVal})
}

func getPageContent(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}

	limit := argInt(args, "limit")
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
		return errResult(err)
	}

	blocks, err := extractAllRecords(data, "block")
	if err != nil {
		return errResult(err)
	}

	var page map[string]any
	var children []map[string]any
	for _, block := range blocks {
		id, _ := block["id"].(string)
		if id == pageID {
			page = block
			continue
		}
		children = append(children, block)
	}
	if page == nil {
		return errResult(fmt.Errorf("page %q not found in response", pageID))
	}

	return jsonResult(map[string]any{
		"page":   page,
		"blocks": children,
	})
}

func createPageWithContent(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	parent := argMap(args, "parent")
	if parent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}

	children, ok := args["children"].([]any)
	if !ok || len(children) == 0 {
		return errResult(fmt.Errorf("children is required"))
	}

	parentID, parentTable := resolveParent(parent)
	if parentID == "" {
		return errResult(fmt.Errorf("parent must contain page_id or database_id"))
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

	if props := argMap(args, "properties"); props != nil {
		blockData["properties"] = props
	}
	if title := argStr(args, "title"); title != "" {
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
		// Link page to parent
		buildListAfterOp("block", parentID, []string{"content"}, map[string]any{
			"id": pageID,
		}),
	}

	// Create child blocks
	for _, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		childOps := buildChildBlockOps(n, pageID, childMap, now)
		ops = append(ops, childOps...)
	}

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{"id": pageID, "url": notionPageURL(pageID)})
}
