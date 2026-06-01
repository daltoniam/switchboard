package notion

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Search (v1) ---
//
// v1 search is scoped to pages that have been shared with the integration
// — there is no implicit workspace-wide visibility like the v3 cookie
// backend has. Callers querying "is page X here?" need to either pre-share
// it or instruct the user to do so via Notion's "Add Connections" menu.

func v1Search(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	typeFilter := r.Str("type")
	limit := r.Int("limit")
	sortArg := r.Map("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{}
	if query != "" {
		body["query"] = query
	}
	if typeFilter != "" {
		// Notion v1 expects: filter: {property: "object", value: "page" | "data_source"}
		// The v3 tool accepted "page" or "data_source" — translate.
		value := typeFilter
		if value == "database" {
			value = "data_source"
		}
		body["filter"] = map[string]any{"property": "object", "value": value}
	}
	if sortArg != nil {
		// v1 sort accepts {direction: "ascending"|"descending", timestamp: "last_edited_time"}
		body["sort"] = sortArg
	} else {
		body["sort"] = map[string]any{"direction": "descending", "timestamp": "last_edited_time"}
	}
	if limit > 0 {
		if limit > 100 {
			limit = 100
		}
		body["page_size"] = limit
	}

	data, err := n.post(ctx, "/search", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Users (v1) ---

func v1ListUsers(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	_ = args
	data, err := n.get(ctx, "/users")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1RetrieveUser(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	userID, err := mcp.ArgStr(args, "user_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if userID == "" {
		return mcp.ErrResult(fmt.Errorf("user_id is required"))
	}
	data, err := n.get(ctx, "/users/%s", userID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1GetSelf(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	_ = args
	data, err := n.get(ctx, "/users/me")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Comments (v1) ---

func v1CreateComment(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
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

	body := map[string]any{
		"rich_text": v1RichTextFromString(text),
	}
	if discussionID != "" {
		body["discussion_id"] = discussionID
	} else {
		body["parent"] = map[string]any{"page_id": pageID}
	}

	data, err := n.post(ctx, "/comments", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1RetrieveComments(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
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
		"block_id":     blockID,
		"start_cursor": startCursor,
		"page_size":    fmt.Sprintf("%d", pageSize),
	})
	data, err := n.get(ctx, "/comments%s", qs)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Tiny shared helper ---
//
// jsonUnmarshalLite is the smallest possible wrapper around json.Unmarshal
// so v1 handlers don't have to reach for the legacy unmarshalJSON helper
// (which exists in the v3 codepath but has no special semantics worth
// sharing).
func jsonUnmarshalLite(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
