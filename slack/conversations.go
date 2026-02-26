package slack

import (
	"context"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

func listConversations(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	types := argStr(args, "types")
	if types == "" {
		types = "public_channel,private_channel"
	}
	params := &slack.GetConversationsParameters{
		Types:           strings.Split(types, ","),
		Limit:           optInt(args, "limit", 100),
		Cursor:          argStr(args, "cursor"),
		ExcludeArchived: true,
	}
	if v, ok := args["exclude_archived"]; ok {
		params.ExcludeArchived = argBool(map[string]any{"v": v}, "v")
	}

	channels, cursor, err := s.getClient().GetConversationsContext(ctx, params)
	if err != nil {
		return errResult(err), nil
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

	return jsonResult(map[string]any{
		"count":         len(out),
		"conversations": out,
		"next_cursor":   cursor,
	})
}

func getConversationInfo(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ch, err := s.getClient().GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID:         argStr(args, "channel_id"),
		IncludeNumMembers: true,
	})
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
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
	params := &slack.GetConversationHistoryParameters{
		ChannelID: argStr(args, "channel_id"),
		Limit:     optInt(args, "limit", 50),
		Oldest:    argStr(args, "oldest"),
		Latest:    argStr(args, "latest"),
		Cursor:    argStr(args, "cursor"),
		Inclusive: true,
	}

	resp, err := s.getClient().GetConversationHistoryContext(ctx, params)
	if err != nil {
		return errResult(err), nil
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

	return jsonResult(map[string]any{
		"channel":     argStr(args, "channel_id"),
		"count":       len(msgs),
		"has_more":    resp.HasMore,
		"messages":    msgs,
		"next_cursor": resp.ResponseMetaData.NextCursor,
	})
}

func getThread(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	params := &slack.GetConversationRepliesParameters{
		ChannelID: argStr(args, "channel_id"),
		Timestamp: argStr(args, "thread_ts"),
	}

	msgs, _, _, err := s.getClient().GetConversationRepliesContext(ctx, params)
	if err != nil {
		return errResult(err), nil
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
			IsParent: m.Timestamp == argStr(args, "thread_ts"),
		})
	}

	return jsonResult(map[string]any{
		"channel":   argStr(args, "channel_id"),
		"thread_ts": argStr(args, "thread_ts"),
		"count":     len(out),
		"messages":  out,
	})
}

func createConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ch, err := s.getClient().CreateConversationContext(ctx, slack.CreateConversationParams{
		ChannelName: argStr(args, "name"),
		IsPrivate:   argBool(args, "is_private"),
	})
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{
		"id":         ch.ID,
		"name":       ch.Name,
		"is_private": ch.IsPrivate,
		"created":    ch.Created,
	})
}

func archiveConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	err := s.getClient().ArchiveConversationContext(ctx, argStr(args, "channel_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "archived", "channel_id": argStr(args, "channel_id")})
}

func inviteToConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	users := strings.Split(argStr(args, "user_ids"), ",")
	ch, err := s.getClient().InviteUsersToConversationContext(ctx, argStr(args, "channel_id"), users...)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "invited", "channel": ch.Name, "users": users})
}

func kickFromConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	err := s.getClient().KickUserFromConversationContext(ctx, argStr(args, "channel_id"), argStr(args, "user_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "removed", "channel_id": argStr(args, "channel_id"), "user_id": argStr(args, "user_id")})
}

func setConversationTopic(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ch, err := s.getClient().SetTopicOfConversationContext(ctx, argStr(args, "channel_id"), argStr(args, "topic"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"channel": ch.Name, "topic": ch.Topic.Value})
}

func setConversationPurpose(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ch, err := s.getClient().SetPurposeOfConversationContext(ctx, argStr(args, "channel_id"), argStr(args, "purpose"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"channel": ch.Name, "purpose": ch.Purpose.Value})
}

func joinConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ch, _, _, err := s.getClient().JoinConversationContext(ctx, argStr(args, "channel_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "joined", "channel": ch.Name})
}

func leaveConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := s.getClient().LeaveConversationContext(ctx, argStr(args, "channel_id"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "left", "channel_id": argStr(args, "channel_id")})
}

func renameConversation(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ch, err := s.getClient().RenameConversationContext(ctx, argStr(args, "channel_id"), argStr(args, "name"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"id": ch.ID, "name": ch.Name})
}
