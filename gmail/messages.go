package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// ── Profile ─────────────────────────────────────────────────────────

func getProfile(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/profile", user(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── Messages ────────────────────────────────────────────────────────

func listMessages(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"q":                 argStr(args, "q"),
		"maxResults":        argStr(args, "max_results"),
		"pageToken":         argStr(args, "page_token"),
		"includeSpamTrash":  argStr(args, "include_spam_trash"),
	}
	if ids := argStr(args, "label_ids"); ids != "" {
		for _, id := range strings.Split(ids, ",") {
			params["labelIds"] = strings.TrimSpace(id)
		}
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/messages%s", user(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"format": argStr(args, "format"),
	}
	if hdrs := argStr(args, "metadata_headers"); hdrs != "" {
		params["metadataHeaders"] = hdrs
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/messages/%s%s", user(args), argStr(args, "message_id"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func buildRawMessage(args map[string]any) string {
	if raw := argStr(args, "raw"); raw != "" {
		return raw
	}
	to := argStr(args, "to")
	subject := argStr(args, "subject")
	body := argStr(args, "body")
	if to == "" && subject == "" && body == "" {
		return ""
	}
	var msg strings.Builder
	if to != "" {
		msg.WriteString("To: " + to + "\r\n")
	}
	if subject != "" {
		msg.WriteString("Subject: " + subject + "\r\n")
	}
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	return base64.URLEncoding.EncodeToString([]byte(msg.String()))
}

func sendMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	raw := buildRawMessage(args)
	body := map[string]any{"raw": raw}
	if tid := argStr(args, "thread_id"); tid != "" {
		body["threadId"] = tid
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/send", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.del(ctx, "/gmail/v1/users/%s/messages/%s", user(args), argStr(args, "message_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func trashMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/%s/trash", user(args), argStr(args, "message_id"))
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func untrashMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/%s/untrash", user(args), argStr(args, "message_id"))
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func modifyMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if ids := argStrSlice(args, "add_label_ids"); len(ids) > 0 {
		body["addLabelIds"] = ids
	}
	if ids := argStrSlice(args, "remove_label_ids"); len(ids) > 0 {
		body["removeLabelIds"] = ids
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/%s/modify", user(args), argStr(args, "message_id"))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func batchModifyMessages(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"ids": argStrSlice(args, "message_ids"),
	}
	if ids := argStrSlice(args, "add_label_ids"); len(ids) > 0 {
		body["addLabelIds"] = ids
	}
	if ids := argStrSlice(args, "remove_label_ids"); len(ids) > 0 {
		body["removeLabelIds"] = ids
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/batchModify", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func batchDeleteMessages(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"ids": argStrSlice(args, "message_ids"),
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/batchDelete", user(args))
	data, err := g.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getAttachment(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	data, err := g.get(ctx, "/gmail/v1/users/%s/messages/%s/attachments/%s",
		user(args), argStr(args, "message_id"), argStr(args, "attachment_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// ── History ─────────────────────────────────────────────────────────

func listHistory(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"startHistoryId": argStr(args, "start_history_id"),
		"labelId":        argStr(args, "label_id"),
		"maxResults":     argStr(args, "max_results"),
		"pageToken":      argStr(args, "page_token"),
	}
	if types := argStr(args, "history_types"); types != "" {
		for _, t := range strings.Split(types, ",") {
			params["historyTypes"] = strings.TrimSpace(t)
		}
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/history%s", user(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}


