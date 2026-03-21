package slack

import (
	"context"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

func listConversations(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	types := r.Str("types")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if types == "" {
		types = "public_channel,private_channel"
	}
	params := &slack.GetConversationsParameters{
		Types:           strings.Split(types, ","),
		Limit:           mcp.OptInt(args, "limit", 100),
		Cursor:          cursor,
		ExcludeArchived: true,
	}
	if v, ok := args["exclude_archived"]; ok {
		b, err := mcp.ArgBool(map[string]any{"v": v}, "v")
		if err != nil {
			return mcp.ErrResult(err)
		}
		params.ExcludeArchived = b
	}

	channels, nextCursor, err := s.getClient().GetConversationsContext(ctx, params)
	if err != nil {
		return errResult(err)
	}

	type ch struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Type       string `json:"type"`
		NumMembers int    `json:"num_members,omitempty"`
		Topic      string `json:"topic,omitempty"`
		Purpose    string `json:"purpose,omitempty"`
		IsArchived bool   `json:"is_archived,omitempty"`
	}
	out := make([]ch, 0, len(channels))
	for _, c := range channels {
		t := "public_channel"
		if c.IsIM {
			t = "im"
		} else if c.IsMpIM {
			t = "mpim"
		} else if c.IsPrivate {
			t = "private_channel"
		}
		out = append(out, ch{
			ID:         c.ID,
			Name:       c.Name,
			Type:       t,
			NumMembers: c.NumMembers,
			Topic:      c.Topic.Value,
			Purpose:    c.Purpose.Value,
			IsArchived: c.IsArchived,
		})
	}

	return mcp.JSONResult(map[string]any{
		"count":         len(out),
		"conversations": out,
		"next_cursor":   nextCursor,
	})
}

func getConversationInfo(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ch, err := s.getClient().GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID:         channelID,
		IncludeNumMembers: true,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"id":          ch.ID,
		"name":        ch.Name,
		"is_channel":  ch.IsChannel,
		"is_private":  ch.IsPrivate,
		"is_im":       ch.IsIM,
		"is_mpim":     ch.IsMpIM,
		"is_archived": ch.IsArchived,
		"num_members": ch.NumMembers,
		"topic":       ch.Topic.Value,
		"purpose":     ch.Purpose.Value,
		"creator":     ch.Creator,
		"created":     ch.Created,
	})
}

func conversationsHistory(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	oldest := r.Str("oldest")
	latest := r.Str("latest")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Limit:     mcp.OptInt(args, "limit", 50),
		Oldest:    oldest,
		Latest:    latest,
		Cursor:    cursor,
		Inclusive: true,
	}

	resp, err := s.getClient().GetConversationHistoryContext(ctx, params)
	if err != nil {
		return errResult(err)
	}

	type msg struct {
		TS         string `json:"ts"`
		User       string `json:"user"`
		Text       string `json:"text"`
		ThreadTS   string `json:"thread_ts,omitempty"`
		ReplyCount int    `json:"reply_count,omitempty"`
	}
	msgs := make([]msg, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		msgs = append(msgs, msg{
			TS:         m.Timestamp,
			User:       m.User,
			Text:       m.Text,
			ThreadTS:   m.ThreadTimestamp,
			ReplyCount: m.ReplyCount,
		})
	}

	return mcp.JSONResult(map[string]any{
		"channel":     channelID,
		"count":       len(msgs),
		"has_more":    resp.HasMore,
		"messages":    msgs,
		"next_cursor": resp.ResponseMetaData.NextCursor,
	})
}

func getThread(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	threadTS := r.Str("thread_ts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := &slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
	}

	msgs, _, _, err := s.getClient().GetConversationRepliesContext(ctx, params)
	if err != nil {
		return errResult(err)
	}

	type reply struct {
		TS       string `json:"ts"`
		User     string `json:"user"`
		Text     string `json:"text"`
		IsParent bool   `json:"is_parent,omitempty"`
	}
	out := make([]reply, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, reply{
			TS:       m.Timestamp,
			User:     m.User,
			Text:     m.Text,
			IsParent: m.Timestamp == threadTS,
		})
	}

	return mcp.JSONResult(map[string]any{
		"channel":   channelID,
		"thread_ts": threadTS,
		"count":     len(out),
		"messages":  out,
	})
}

func createConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	isPrivate := r.Bool("is_private")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ch, err := s.getClient().CreateConversationContext(ctx, slack.CreateConversationParams{
		ChannelName: name,
		IsPrivate:   isPrivate,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"id":         ch.ID,
		"name":       ch.Name,
		"is_private": ch.IsPrivate,
		"created":    ch.Created,
	})
}

func archiveConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	err := s.getClient().ArchiveConversationContext(ctx, channelID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "archived", "channel_id": channelID})
}

func inviteToConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	userIDs := r.Str("user_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	users := strings.Split(userIDs, ",")
	ch, err := s.getClient().InviteUsersToConversationContext(ctx, channelID, users...)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "invited", "channel": ch.Name, "users": users})
}

func kickFromConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	userID := r.Str("user_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	err := s.getClient().KickUserFromConversationContext(ctx, channelID, userID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "removed", "channel_id": channelID, "user_id": userID})
}

func setConversationTopic(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	topic := r.Str("topic")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ch, err := s.getClient().SetTopicOfConversationContext(ctx, channelID, topic)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"channel": ch.Name, "topic": ch.Topic.Value})
}

func setConversationPurpose(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	purpose := r.Str("purpose")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ch, err := s.getClient().SetPurposeOfConversationContext(ctx, channelID, purpose)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"channel": ch.Name, "purpose": ch.Purpose.Value})
}

func joinConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ch, _, _, err := s.getClient().JoinConversationContext(ctx, channelID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "joined", "channel": ch.Name})
}

func leaveConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := s.getClient().LeaveConversationContext(ctx, channelID)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "left", "channel_id": channelID})
}

func renameConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ch, err := s.getClient().RenameConversationContext(ctx, channelID, name)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"id": ch.ID, "name": ch.Name})
}
