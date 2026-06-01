package notion

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Blocks (v1) ---

func v1RetrieveBlock(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	blockID, err := mcp.ArgStr(args, "block_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}
	data, err := n.get(ctx, "/blocks/%s", blockID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// v1UpdateBlock translates the v3 `type_content` payload into the v1 shape.
// v3 callers send: {"properties": {"title": [["new text"]]}}
// v1 expects: {"<block_type>": {"rich_text": [{"type":"text","text":{"content":"new text"}}]}}
//
// To translate we need to know the block's current type, so we fetch it
// first. Power-users can bypass the translation by providing a pre-shaped
// v1 payload with the block type as the top-level key (e.g.
// {"paragraph": {"rich_text": [...]}}) — that pass-through preserves the
// full v1 expressiveness (annotations, links, mentions, etc).
func v1UpdateBlock(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	blockID := r.Str("block_id")
	typeContent := r.Map("type_content")
	archived := r.Bool("archived")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}

	body := map[string]any{}
	if archived {
		body["archived"] = true
	}

	if typeContent != nil {
		// Pass-through: caller supplied a v1-shaped payload.
		passedThrough := false
		for k, v := range typeContent {
			if k == "properties" || k == "format" {
				continue
			}
			body[k] = v
			passedThrough = true
		}

		// Legacy v3 path: translate properties.title to v1 rich_text.
		if !passedThrough {
			blockType, err := v1FetchBlockType(ctx, n, blockID)
			if err != nil {
				return mcp.ErrResult(err)
			}
			text := v1ExtractV3TitleText(map[string]any{"properties": typeContent["properties"]})
			payload := map[string]any{"rich_text": v1RichTextFromString(text)}
			body[blockType] = payload
		}
	}

	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("at least one of type_content or archived must be set"))
	}

	data, err := n.patch(ctx, "/blocks/"+blockID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1DeleteBlock(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	blockID, err := mcp.ArgStr(args, "block_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}
	data, err := n.del(ctx, "/blocks/%s", blockID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1GetBlockChildren(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	blockID := r.Str("block_id")
	startCursor := r.Str("start_cursor")
	pageSize := r.Int("page_size")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}
	qs := queryEncode(map[string]string{
		"start_cursor": startCursor,
		"page_size":    fmt.Sprintf("%d", pageSize),
	})
	data, err := n.get(ctx, "/blocks/%s/children%s", blockID, qs)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1AppendBlockChildren(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	blockID := r.Str("block_id")
	after := r.Str("after")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}
	childrenAny, ok := args["children"].([]any)
	if !ok || len(childrenAny) == 0 {
		return mcp.ErrResult(fmt.Errorf("children is required"))
	}
	if len(childrenAny) > 100 {
		return mcp.ErrResult(fmt.Errorf("v1 append_block_children accepts up to 100 children per call; got %d. Split into multiple calls", len(childrenAny)))
	}

	normalized := make([]any, 0, len(childrenAny))
	for _, c := range childrenAny {
		if cm, ok := c.(map[string]any); ok {
			normalized = append(normalized, v1NormalizeBlock(cm))
		}
	}

	body := map[string]any{"children": normalized}
	if after != "" {
		body["after"] = after
	}
	data, err := n.patch(ctx, "/blocks/"+blockID+"/children", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// v1FetchBlockType issues a GET /blocks/{id} and returns the block's
// "type" field. Used by update_block to translate the v3 properties.title
// payload into a v1 rich_text payload keyed by the block's type.
func v1FetchBlockType(ctx context.Context, n *notionV1, blockID string) (string, error) {
	data, err := n.get(ctx, "/blocks/%s", blockID)
	if err != nil {
		return "", err
	}
	var b struct {
		Type string `json:"type"`
	}
	if err := jsonUnmarshalLite(data, &b); err != nil {
		return "", err
	}
	if b.Type == "" {
		return "", fmt.Errorf("could not determine block type for %s", blockID)
	}
	return b.Type, nil
}
