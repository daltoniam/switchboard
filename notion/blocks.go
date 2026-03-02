package notion

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func retrieveBlock(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return errResult(fmt.Errorf("block_id is required"))
	}
	data, err := n.get(ctx, "/v1/blocks/%s", blockID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateBlock(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return errResult(fmt.Errorf("block_id is required"))
	}

	body := map[string]any{}
	if v := argMap(args, "type_content"); v != nil {
		for k, val := range v {
			body[k] = val
		}
	}
	if _, ok := args["archived"]; ok {
		body["archived"] = argBool(args, "archived")
	}

	data, err := n.patch(ctx, fmt.Sprintf("/v1/blocks/%s", blockID), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteBlock(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return errResult(fmt.Errorf("block_id is required"))
	}
	data, err := n.del(ctx, "/v1/blocks/%s", blockID)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getBlockChildren(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return errResult(fmt.Errorf("block_id is required"))
	}

	vals := url.Values{}
	if v := argStr(args, "start_cursor"); v != "" {
		vals.Set("start_cursor", v)
	}
	if v := argInt(args, "page_size"); v > 0 {
		vals.Set("page_size", strconv.Itoa(v))
	}

	path := fmt.Sprintf("/v1/blocks/%s/children", blockID)
	if len(vals) > 0 {
		path += "?" + vals.Encode()
	}

	data, err := n.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func appendBlockChildren(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return errResult(fmt.Errorf("block_id is required"))
	}
	children := args["children"]
	if children == nil {
		return errResult(fmt.Errorf("children is required"))
	}

	body := map[string]any{
		"children": children,
	}

	data, err := n.patch(ctx, fmt.Sprintf("/v1/blocks/%s/children", blockID), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
