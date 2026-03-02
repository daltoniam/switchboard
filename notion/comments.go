package notion

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

func createComment(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	richText := args["rich_text"]
	if richText == nil {
		return errResult(fmt.Errorf("rich_text is required"))
	}

	body := map[string]any{
		"rich_text": richText,
	}
	if v := argMap(args, "parent"); v != nil {
		body["parent"] = v
	}
	if v := argStr(args, "discussion_id"); v != "" {
		body["discussion_id"] = v
	}

	data, err := n.post(ctx, "/v1/comments", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func retrieveComments(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID := argStr(args, "block_id")
	if blockID == "" {
		return errResult(fmt.Errorf("block_id is required"))
	}

	vals := url.Values{}
	vals.Set("block_id", blockID)
	if v := argStr(args, "start_cursor"); v != "" {
		vals.Set("start_cursor", v)
	}
	if v := argInt(args, "page_size"); v > 0 {
		vals.Set("page_size", strconv.Itoa(v))
	}

	data, err := n.doRequest(ctx, "GET", "/v1/comments?"+vals.Encode(), nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
