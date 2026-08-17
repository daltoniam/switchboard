package notion

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func createComment(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	text := r.Str("text")
	pageID := r.Str("page_id")
	discussionID := r.Str("discussion_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if text == "" {
		return mcp.ErrResult(fmt.Errorf("text is required"))
	}
	if pageID == "" && discussionID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id or discussion_id is required"))
	}

	commentID := newBlockID()
	now := currentTimeMillis()

	var ops []op

	if discussionID == "" {
		// New discussion on a page
		discussionID = newBlockID()
		discData := map[string]any{
			"id":           discussionID,
			"parent_id":    pageID,
			"parent_table": "block",
			"space_id":     n.spaceID,
			"resolved":     false,
			"alive":        true,
		}
		ops = append(ops,
			buildSetOp("discussion", discussionID, []string{}, discData),
			buildListAfterOp("block", pageID, []string{"discussions"}, map[string]any{
				"id": discussionID,
			}),
		)
	}

	commentData := map[string]any{
		"id":               commentID,
		"parent_id":        discussionID,
		"parent_table":     "discussion",
		"space_id":         n.spaceID,
		"text":             []any{[]any{text}},
		"created_by_id":    n.userID,
		"created_by_table": "notion_user",
		"created_time":     now,
		"alive":            true,
	}

	ops = append(ops,
		buildSetOp("comment", commentID, []string{}, commentData),
		buildListAfterOp("discussion", discussionID, []string{"comments"}, map[string]any{
			"id": commentID,
		}),
	)

	_, err := submitTransaction(ctx, n, ops)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"id":            commentID,
		"discussion_id": discussionID,
	})
}

func retrieveComments(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	blockID, err := mcp.ArgStr(args, "block_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if blockID == "" {
		return mcp.ErrResult(fmt.Errorf("block_id is required"))
	}

	// Load the page chunk — comments are bundled as discussion + comment records
	data, err := n.doRequest(ctx, "/api/v3/loadCachedPageChunkV2", map[string]any{
		"page":            map[string]any{"id": blockID},
		"limit":           100,
		"cursor":          map[string]any{"stack": []any{}},
		"verticalColumns": false,
	})
	if err != nil {
		return mcp.ErrResult(err)
	}

	// Extract discussions and comments from recordMap.
	// Pages with no comments have no "discussion" or "comment" tables — not an error.
	discussions, _ := extractAllRecords(data, "discussion")
	comments, _ := extractAllRecords(data, "comment")

	// Build discussion → comments mapping
	discComments := map[string][]map[string]any{}
	for _, c := range comments {
		parentID, _ := c["parent_id"].(string)
		discComments[parentID] = append(discComments[parentID], c)
	}

	// Build result: discussions with their comments
	results := make([]map[string]any, 0, len(discussions))
	for _, d := range discussions {
		discID, _ := d["id"].(string)
		entry := map[string]any{
			"discussion": d,
			"comments":   discComments[discID],
		}
		results = append(results, entry)
	}

	return mcp.JSONResult(map[string]any{"results": results})
}
