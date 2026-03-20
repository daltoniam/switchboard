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
	r := mcp.NewArgs(args)
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/profile", u)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Messages ────────────────────────────────────────────────────────

func listMessages(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"q":                r.Str("q"),
		"maxResults":       r.Str("max_results"),
		"pageToken":        r.Str("page_token"),
		"includeSpamTrash": r.Str("include_spam_trash"),
	}
	var multi map[string][]string
	if ids := r.Str("label_ids"); ids != "" {
		multi = map[string][]string{"labelIds": r.StrSlice("label_ids")}
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncodeMulti(params, multi)
	data, err := g.get(ctx, "/gmail/v1/users/%s/messages%s", u, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	format := r.Str("format")
	hdrs := r.Str("metadata_headers")
	u := user(r)
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{"format": format}
	if hdrs != "" {
		params["metadataHeaders"] = hdrs
	}
	q := queryEncode(params)
	data, err := g.get(ctx, "/gmail/v1/users/%s/messages/%s%s", u, msgID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func sanitizeHeader(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", "")
}

func buildRawMessage(r *mcp.Args) string {
	if raw := r.Str("raw"); raw != "" {
		return raw
	}
	to := sanitizeHeader(r.Str("to"))
	from := sanitizeHeader(r.Str("from"))
	subject := sanitizeHeader(r.Str("subject"))
	body := r.Str("body")
	if to == "" && subject == "" && body == "" {
		return ""
	}
	var msg strings.Builder
	if from != "" {
		msg.WriteString("From: " + from + "\r\n")
	}
	if to != "" {
		msg.WriteString("To: " + to + "\r\n")
	}
	if subject != "" {
		msg.WriteString("Subject: " + subject + "\r\n")
	}
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	return base64.RawURLEncoding.EncodeToString([]byte(msg.String()))
}

func sendMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	raw := buildRawMessage(r)
	body := map[string]any{"raw": raw}
	if tid := r.Str("thread_id"); tid != "" {
		body["threadId"] = tid
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/send", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.del(ctx, "/gmail/v1/users/%s/messages/%s", u, msgID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func trashMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/%s/trash", u, msgID)
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func untrashMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/%s/untrash", u, msgID)
	data, err := g.post(ctx, path, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func modifyMessage(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{}
	if ids := r.StrSlice("add_label_ids"); len(ids) > 0 {
		body["addLabelIds"] = ids
	}
	if ids := r.StrSlice("remove_label_ids"); len(ids) > 0 {
		body["removeLabelIds"] = ids
	}
	u := user(r)
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/%s/modify", u, msgID)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func batchModifyMessages(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"ids": r.StrSlice("message_ids"),
	}
	if ids := r.StrSlice("add_label_ids"); len(ids) > 0 {
		body["addLabelIds"] = ids
	}
	if ids := r.StrSlice("remove_label_ids"); len(ids) > 0 {
		body["removeLabelIds"] = ids
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/batchModify", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func batchDeleteMessages(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	body := map[string]any{
		"ids": r.StrSlice("message_ids"),
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/gmail/v1/users/%s/messages/batchDelete", u)
	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAttachment(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	u := user(r)
	msgID := r.Str("message_id")
	attID := r.Str("attachment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := g.get(ctx, "/gmail/v1/users/%s/messages/%s/attachments/%s", u, msgID, attID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── History ─────────────────────────────────────────────────────────

func listHistory(ctx context.Context, g *gmail, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	params := map[string]string{
		"startHistoryId": r.Str("start_history_id"),
		"labelId":        r.Str("label_id"),
		"maxResults":     r.Str("max_results"),
		"pageToken":      r.Str("page_token"),
	}
	var multi map[string][]string
	if types := r.Str("history_types"); types != "" {
		multi = map[string][]string{"historyTypes": r.StrSlice("history_types")}
	}
	u := user(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncodeMulti(params, multi)
	data, err := g.get(ctx, "/gmail/v1/users/%s/history%s", u, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
