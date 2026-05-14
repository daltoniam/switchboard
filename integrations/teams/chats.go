package teams

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// listChats -> GET /me/chats
func listChats(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	top := r.OptInt("top", 20)
	filter := r.Str("filter")
	orderby := r.Str("orderby")
	expand := r.Str("expand")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if top > 50 {
		top = 50
	}
	q := url.Values{}
	q.Set("$top", fmt.Sprintf("%d", top))
	if filter != "" {
		q.Set("$filter", filter)
	}
	if orderby != "" {
		q.Set("$orderby", orderby)
	}
	if expand != "" {
		q.Set("$expand", expand)
	}
	data, err := t.graphGet(ctx, tn.TenantID, "/me/chats?"+q.Encode())
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// getChat -> GET /chats/{id}
func getChat(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	chatID := r.Str("chat_id")
	expand := r.Str("expand")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if chatID == "" {
		return mcp.ErrResult(fmt.Errorf("chat_id is required"))
	}
	path := "/chats/" + url.PathEscape(chatID)
	if expand != "" {
		path += "?$expand=" + url.QueryEscape(expand)
	}
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// listChatMessages -> GET /chats/{id}/messages
func listChatMessages(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	chatID := r.Str("chat_id")
	top := r.OptInt("top", 20)
	orderby := r.Str("orderby")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if chatID == "" {
		return mcp.ErrResult(fmt.Errorf("chat_id is required"))
	}
	if top > 50 {
		top = 50
	}
	q := url.Values{}
	q.Set("$top", fmt.Sprintf("%d", top))
	if orderby != "" {
		q.Set("$orderby", orderby)
	}
	path := fmt.Sprintf("/chats/%s/messages?%s", url.PathEscape(chatID), q.Encode())
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// getChatMessage -> GET /chats/{id}/messages/{mid}
func getChatMessage(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	chatID := r.Str("chat_id")
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if chatID == "" || msgID == "" {
		return mcp.ErrResult(fmt.Errorf("chat_id and message_id are required"))
	}
	path := fmt.Sprintf("/chats/%s/messages/%s", url.PathEscape(chatID), url.PathEscape(msgID))
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// sendChatMessage -> POST /chats/{id}/messages
func sendChatMessage(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	chatID := r.Str("chat_id")
	content := r.Str("content")
	contentType := r.Str("content_type")
	subject := r.Str("subject")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if chatID == "" || content == "" {
		return mcp.ErrResult(fmt.Errorf("chat_id and content are required"))
	}
	body := map[string]any{
		"body": map[string]any{
			"contentType": formatContentType(contentType),
			"content":     content,
		},
	}
	if subject != "" {
		body["subject"] = subject
	}
	path := fmt.Sprintf("/chats/%s/messages", url.PathEscape(chatID))
	data, err := t.graphPost(ctx, tn.TenantID, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// listChatMembers -> GET /chats/{id}/members
func listChatMembers(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	chatID := r.Str("chat_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if chatID == "" {
		return mcp.ErrResult(fmt.Errorf("chat_id is required"))
	}
	path := fmt.Sprintf("/chats/%s/members", url.PathEscape(chatID))
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
