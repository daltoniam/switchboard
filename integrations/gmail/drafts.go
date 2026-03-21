package gmail

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDrafts(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"q":                r.Str("q"),
		"maxResults":       r.Str("max_results"),
		"pageToken":        r.Str("page_token"),
		"includeSpamTrash": r.Str("include_spam_trash"),
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/drafts%s", u, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"format": r.Str("format"),
	}
	u := user(r)
	draftID := r.Str("draft_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/drafts/%s%s", u, draftID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	msg := map[string]any{}
	if raw := buildRawMessage(r); raw != "" {
		msg["raw"] = raw
	}
	if tid := r.Str("thread_id"); tid != "" {
		msg["threadId"] = tid
	}
	body := map[string]any{"message": msg}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/drafts", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	msg := map[string]any{}
	if raw := buildRawMessage(r); raw != "" {
		msg["raw"] = raw
	}
	if tid := r.Str("thread_id"); tid != "" {
		msg["threadId"] = tid
	}
	body := map[string]any{"message": msg}
	u := user(r)
	draftID := r.Str("draft_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/drafts/%s", u, draftID)
	data, err := g.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	draftID := r.Str("draft_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/drafts/%s", u, draftID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func sendDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"id": r.Str("draft_id"),
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/drafts/send", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
