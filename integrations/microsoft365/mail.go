package microsoft365

import (
	"context"
	"net/url"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listMessages(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		userID := r.Str("user_id")
		folderID := r.Str("folder_id")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		if params["$orderby"] == "" && params["$search"] == "" {
			params["$orderby"] = "receivedDateTime desc"
		}
		headers := map[string]string{}
		if params["$search"] != "" {
			headers["ConsistencyLevel"] = "eventual"
		}
		base := userPath(userID)
		if folderID != "" {
			base += "/mailFolders/" + url.PathEscape(folderID)
		}
		return base + "/messages" + queryEncode(params), headers, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func getMessage(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	msgID := r.Str("message_id")
	sel := r.Str("select")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.get(ctx, userPath(userID)+"/messages/"+url.PathEscape(msgID)+queryEncode(map[string]string{"$select": sel}))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func messageBody(r *mcp.Args) map[string]any {
	bodyType := r.Str("body_type")
	if bodyType == "" {
		bodyType = "Text"
	}
	msg := map[string]any{
		"subject": r.Str("subject"),
		"body": map[string]any{
			"contentType": bodyType,
			"content":     r.Str("body"),
		},
	}
	if to := splitEmails(r.Str("to")); len(to) > 0 {
		msg["toRecipients"] = to
	}
	if cc := splitEmails(r.Str("cc")); len(cc) > 0 {
		msg["ccRecipients"] = cc
	}
	if bcc := splitEmails(r.Str("bcc")); len(bcc) > 0 {
		msg["bccRecipients"] = bcc
	}
	if imp := r.Str("importance"); imp != "" {
		msg["importance"] = imp
	}
	return msg
}

func sendMail(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	save := r.Str("save_to_sent")
	msg := messageBody(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	saveToSent := save != "false"
	body := map[string]any{
		"message":         msg,
		"saveToSentItems": saveToSent,
	}
	data, err := m.post(ctx, userPath(userID)+"/sendMail", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func createDraft(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	msg := messageBody(r)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.post(ctx, userPath(userID)+"/messages", msg)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func replyMessage(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	msgID := r.Str("message_id")
	comment := r.Str("comment")
	replyAll := r.Str("reply_all")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	action := "reply"
	if strings.EqualFold(replyAll, "true") {
		action = "replyAll"
	}
	body := map[string]any{}
	if comment != "" {
		body["comment"] = comment
	}
	path := userPath(userID) + "/messages/" + url.PathEscape(msgID) + "/" + action
	data, err := m.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func deleteMessage(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.del(ctx, userPath(userID)+"/messages/"+url.PathEscape(msgID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func listMailFolders(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		userID := r.Str("user_id")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return userPath(userID) + "/mailFolders" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}
