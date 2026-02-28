package gmail

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listDrafts(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"q":                argStr(args, "q"),
		"maxResults":       argStr(args, "max_results"),
		"pageToken":        argStr(args, "page_token"),
		"includeSpamTrash": argStr(args, "include_spam_trash"),
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/drafts%s", user(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"format": argStr(args, "format"),
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/drafts/%s%s", user(args), argStr(args, "draft_id"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	msg := map[string]any{}
	if raw := buildRawMessage(args); raw != "" {
		msg["raw"] = raw
	}
	if tid := argStr(args, "thread_id"); tid != "" {
		msg["threadId"] = tid
	}
	body := map[string]any{"message": msg}
	path := fmt.Sprintf("/gmail/v1/users/%s/drafts", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	msg := map[string]any{}
	if raw := buildRawMessage(args); raw != "" {
		msg["raw"] = raw
	}
	if tid := argStr(args, "thread_id"); tid != "" {
		msg["threadId"] = tid
	}
	body := map[string]any{"message": msg}
	path := fmt.Sprintf("/gmail/v1/users/%s/drafts/%s", user(args), argStr(args, "draft_id"))
	data, err := g.put(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/drafts/%s", user(args), argStr(args, "draft_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func sendDraft(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"id": argStr(args, "draft_id"),
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/drafts/send", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
