package microsoft365

import (
	"context"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func chatMessageBody(content, contentType string) map[string]any {
	if contentType == "" {
		contentType = "text"
	}
	return map[string]any{
		"body": map[string]any{
			"contentType": contentType,
			"content":     content,
		},
	}
}

func listTeams(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return "/me/joinedTeams" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func listChannels(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		teamID := r.Str("team_id")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return "/teams/" + url.PathEscape(teamID) + "/channels" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func listChannelMessages(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		teamID := r.Str("team_id")
		channelID := r.Str("channel_id")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return "/teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func sendChannelMessage(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	teamID := r.Str("team_id")
	channelID := r.Str("channel_id")
	content := r.Str("content")
	contentType := r.Str("content_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	path := "/teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages"
	data, err := m.post(ctx, path, chatMessageBody(content, contentType))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func listChats(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return "/me/chats" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func listChatMessages(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	data, err := nextOrGet(ctx, m, args, func() (string, map[string]string, error) {
		r := mcp.NewArgs(args)
		chatID := r.Str("chat_id")
		params := graphListParams(r)
		if err := r.Err(); err != nil {
			return "", nil, err
		}
		return "/chats/" + url.PathEscape(chatID) + "/messages" + queryEncode(params), nil, nil
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}

func sendChatMessage(ctx context.Context, m *m365, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	chatID := r.Str("chat_id")
	content := r.Str("content")
	contentType := r.Str("content_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := m.post(ctx, "/chats/"+url.PathEscape(chatID)+"/messages", chatMessageBody(content, contentType))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return graphResult(data, nil)
}
