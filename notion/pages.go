package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func createPage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	parent := argMap(args, "parent")
	if parent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}

	body := map[string]any{
		"parent": parent,
	}
	if v := argMap(args, "properties"); v != nil {
		body["properties"] = v
	}
	if v, ok := args["children"]; ok && v != nil {
		body["children"] = v
	}
	if v := argMap(args, "icon"); v != nil {
		body["icon"] = v
	}
	if v := argMap(args, "cover"); v != nil {
		body["cover"] = v
	}

	data, err := n.post(ctx, "/v1/pages", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func retrievePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}
	data, err := n.get(ctx, "/v1/pages/%s", pageID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updatePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}

	body := map[string]any{}
	if v := argMap(args, "properties"); v != nil {
		body["properties"] = v
	}
	if v := argMap(args, "icon"); v != nil {
		body["icon"] = v
	}
	if v := argMap(args, "cover"); v != nil {
		body["cover"] = v
	}
	if _, ok := args["archived"]; ok {
		body["archived"] = argBool(args, "archived")
	}

	data, err := n.patch(ctx, fmt.Sprintf("/v1/pages/%s", pageID), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func movePage(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}
	parent := argMap(args, "parent")
	if parent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}

	body := map[string]any{
		"parent": parent,
	}

	data, err := n.post(ctx, fmt.Sprintf("/v1/pages/%s/move", pageID), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
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

	vals := url.Values{}
	if v := argStr(args, "start_cursor"); v != "" {
		vals.Set("start_cursor", v)
	}
	if v := argInt(args, "page_size"); v > 0 {
		vals.Set("page_size", strconv.Itoa(v))
	}

	path := fmt.Sprintf("/v1/pages/%s/properties/%s", url.PathEscape(pageID), url.PathEscape(propertyID))
	if len(vals) > 0 {
		path += "?" + vals.Encode()
	}

	data, err := n.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Convenience handlers ---

func getPageContent(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	pageID := argStr(args, "page_id")
	if pageID == "" {
		return errResult(fmt.Errorf("page_id is required"))
	}

	maxDepth := argInt(args, "max_depth")
	if maxDepth <= 0 {
		maxDepth = 3
	}

	page, err := n.get(ctx, "/v1/pages/%s", pageID)
	if err != nil {
		return errResult(err)
	}

	blocks, truncated, err := fetchBlocksRecursive(ctx, n, pageID, maxDepth, 0)
	if err != nil {
		return errResult(err)
	}

	result := map[string]json.RawMessage{
		"page":   page,
		"blocks": blocks,
	}
	if truncated {
		result["truncated"] = json.RawMessage("true")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

const maxBlockFetches = 100 // safety limit on total API calls during recursive fetch

func fetchBlocksRecursive(ctx context.Context, n *notion, blockID string, maxDepth, depth int) (json.RawMessage, bool, error) {
	remaining := maxBlockFetches
	data, err := fetchBlocksRecursiveInner(ctx, n, blockID, maxDepth, depth, &remaining)
	return data, remaining <= 0, err
}

func fetchBlocksRecursiveInner(ctx context.Context, n *notion, blockID string, maxDepth, depth int, remaining *int) (json.RawMessage, error) {
	if depth >= maxDepth {
		return json.RawMessage("[]"), nil
	}

	var allBlocks []json.RawMessage
	var cursor string

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if *remaining <= 0 {
			break
		}
		*remaining--
		path := fmt.Sprintf("/v1/blocks/%s/children", blockID)
		vals := url.Values{}
		if cursor != "" {
			vals.Set("start_cursor", cursor)
		}
		if len(vals) > 0 {
			path += "?" + vals.Encode()
		}

		data, err := n.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Results   []json.RawMessage `json:"results"`
			HasMore   bool              `json:"has_more"`
			NextCursor string           `json:"next_cursor"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, err
		}

		for _, block := range resp.Results {
			var meta struct {
				ID          string `json:"id"`
				HasChildren bool   `json:"has_children"`
			}
			if err := json.Unmarshal(block, &meta); err != nil {
				return nil, err
			}

			if meta.HasChildren && depth+1 < maxDepth {
				children, err := fetchBlocksRecursiveInner(ctx, n, meta.ID, maxDepth, depth+1, remaining)
				if err != nil {
					return nil, err
				}
				// Merge children into the block object
				var blockMap map[string]json.RawMessage
				if err := json.Unmarshal(block, &blockMap); err != nil {
					return nil, err
				}
				blockMap["children"] = children
				merged, err := json.Marshal(blockMap)
				if err != nil {
					return nil, err
				}
				allBlocks = append(allBlocks, merged)
			} else {
				allBlocks = append(allBlocks, block)
			}
		}

		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
	}

	if allBlocks == nil {
		return json.RawMessage("[]"), nil
	}
	return json.Marshal(allBlocks)
}

func createPageWithContent(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	parent := argMap(args, "parent")
	if parent == nil {
		return errResult(fmt.Errorf("parent is required"))
	}
	children := args["children"]
	if children == nil {
		return errResult(fmt.Errorf("children is required"))
	}

	body := map[string]any{
		"parent":   parent,
		"children": children,
	}
	if v := argMap(args, "properties"); v != nil {
		body["properties"] = v
	}
	if v := argMap(args, "icon"); v != nil {
		body["icon"] = v
	}
	if v := argMap(args, "cover"); v != nil {
		body["cover"] = v
	}

	data, err := n.post(ctx, "/v1/pages", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
