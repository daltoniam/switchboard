package teams

import (
	"context"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// listJoinedTeams -> GET /me/joinedTeams
func listJoinedTeams(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.graphGet(ctx, tn.TenantID, "/me/joinedTeams")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// listChannels -> GET /teams/{tid}/channels
func listChannels(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" {
		return mcp.ErrResult(fmt.Errorf("team_id is required"))
	}
	path := fmt.Sprintf("/teams/%s/channels", url.PathEscape(teamID))
	if filter != "" {
		path += "?$filter=" + url.QueryEscape(filter)
	}
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// getChannel -> GET /teams/{tid}/channels/{cid}
func getChannel(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" || channelID == "" {
		return mcp.ErrResult(fmt.Errorf("team_id and channel_id are required"))
	}
	path := fmt.Sprintf("/teams/%s/channels/%s", url.PathEscape(teamID), url.PathEscape(channelID))
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// listChannelMessages -> GET /teams/{tid}/channels/{cid}/messages
func listChannelMessages(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	channelID := r.Str("channel_id")
	top := r.OptInt("top", 20)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" || channelID == "" {
		return mcp.ErrResult(fmt.Errorf("team_id and channel_id are required"))
	}
	if top > 50 {
		top = 50
	}
	q := url.Values{}
	q.Set("$top", fmt.Sprintf("%d", top))
	path := fmt.Sprintf("/teams/%s/channels/%s/messages?%s", url.PathEscape(teamID), url.PathEscape(channelID), q.Encode())
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// getChannelMessage -> GET /teams/{tid}/channels/{cid}/messages/{mid}
func getChannelMessage(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	channelID := r.Str("channel_id")
	msgID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" || channelID == "" || msgID == "" {
		return mcp.ErrResult(fmt.Errorf("team_id, channel_id, and message_id are required"))
	}
	path := fmt.Sprintf("/teams/%s/channels/%s/messages/%s",
		url.PathEscape(teamID), url.PathEscape(channelID), url.PathEscape(msgID))
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// listMessageReplies -> GET /teams/{tid}/channels/{cid}/messages/{mid}/replies
func listMessageReplies(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	channelID := r.Str("channel_id")
	msgID := r.Str("message_id")
	top := r.OptInt("top", 20)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" || channelID == "" || msgID == "" {
		return mcp.ErrResult(fmt.Errorf("team_id, channel_id, and message_id are required"))
	}
	if top > 50 {
		top = 50
	}
	q := url.Values{}
	q.Set("$top", fmt.Sprintf("%d", top))
	path := fmt.Sprintf("/teams/%s/channels/%s/messages/%s/replies?%s",
		url.PathEscape(teamID), url.PathEscape(channelID), url.PathEscape(msgID), q.Encode())
	data, err := t.graphGet(ctx, tn.TenantID, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// sendChannelMessage -> POST /teams/{tid}/channels/{cid}/messages
func sendChannelMessage(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	channelID := r.Str("channel_id")
	content := r.Str("content")
	contentType := r.Str("content_type")
	subject := r.Str("subject")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" || channelID == "" || content == "" {
		return mcp.ErrResult(fmt.Errorf("team_id, channel_id, and content are required"))
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
	path := fmt.Sprintf("/teams/%s/channels/%s/messages",
		url.PathEscape(teamID), url.PathEscape(channelID))
	data, err := t.graphPost(ctx, tn.TenantID, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// replyToChannelMessage -> POST /teams/{tid}/channels/{cid}/messages/{mid}/replies
func replyToChannelMessage(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	channelID := r.Str("channel_id")
	msgID := r.Str("message_id")
	content := r.Str("content")
	contentType := r.Str("content_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if teamID == "" || channelID == "" || msgID == "" || content == "" {
		return mcp.ErrResult(fmt.Errorf("team_id, channel_id, message_id, and content are required"))
	}
	body := map[string]any{
		"body": map[string]any{
			"contentType": formatContentType(contentType),
			"content":     content,
		},
	}
	path := fmt.Sprintf("/teams/%s/channels/%s/messages/%s/replies",
		url.PathEscape(teamID), url.PathEscape(channelID), url.PathEscape(msgID))
	data, err := t.graphPost(ctx, tn.TenantID, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
