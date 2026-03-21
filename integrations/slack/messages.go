package slack

import (
	"context"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

var _ *mcp.ToolResult // type anchor

func sendMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	text := r.Str("text")
	threadTS := r.Str("thread_ts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}
	channel, ts, err := client.PostMessageContext(ctx, channelID, opts...)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "sent", "channel": channel, "ts": ts})
}

func updateMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	ts := r.Str("ts")
	text := r.Str("text")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	channel, respTS, _, err := client.UpdateMessageContext(ctx, channelID, ts, opts...)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "updated", "channel": channel, "ts": respTS})
}

func deleteMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	ts := r.Str("ts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	channel, respTS, err := client.DeleteMessageContext(ctx, channelID, ts)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "deleted", "channel": channel, "ts": respTS})
}

func searchMessages(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	query := r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := slack.SearchParameters{Sort: "timestamp", SortDirection: "desc", Count: mcp.OptInt(args, "count", 20)}
	result, err := client.SearchMessagesContext(ctx, query, params)
	if err != nil {
		return errResult(err)
	}
	type match struct {
		TS        string `json:"ts"`
		Channel   string `json:"channel"`
		ChannelID string `json:"channel_id"`
		User      string `json:"user"`
		Text      string `json:"text"`
		Permalink string `json:"permalink"`
	}
	matches := make([]match, 0, len(result.Matches))
	for _, m := range result.Matches {
		matches = append(matches, match{TS: m.Timestamp, Channel: m.Channel.Name, ChannelID: m.Channel.ID, User: m.User, Text: m.Text, Permalink: m.Permalink})
	}
	return mcp.JSONResult(map[string]any{"query": query, "total": result.Total, "matches": matches})
}

func addReaction(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	ts := r.Str("ts")
	emoji := r.Str("emoji")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.AddReactionContext(ctx, emoji, slack.ItemRef{Channel: channelID, Timestamp: ts}); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "added", "emoji": emoji})
}

func removeReaction(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	ts := r.Str("ts")
	emoji := r.Str("emoji")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.RemoveReactionContext(ctx, emoji, slack.ItemRef{Channel: channelID, Timestamp: ts}); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "removed", "emoji": emoji})
}

func getReactions(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	ts := r.Str("ts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	reactedItem, err := client.GetReactionsContext(ctx, slack.ItemRef{Channel: channelID, Timestamp: ts}, slack.GetReactionsParameters{})
	if err != nil {
		return errResult(err)
	}
	type rxn struct {
		Name  string   `json:"name"`
		Count int      `json:"count"`
		Users []string `json:"users"`
	}
	out := make([]rxn, 0, len(reactedItem.Reactions))
	for _, r := range reactedItem.Reactions {
		out = append(out, rxn{Name: r.Name, Count: r.Count, Users: r.Users})
	}
	return mcp.JSONResult(out)
}

func addPin(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	ts := r.Str("ts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.AddPinContext(ctx, channelID, slack.ItemRef{Channel: channelID, Timestamp: ts}); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "pinned"})
}

func removePin(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	ts := r.Str("ts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := client.RemovePinContext(ctx, channelID, slack.ItemRef{Channel: channelID, Timestamp: ts}); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{"status": "unpinned"})
}

func listPins(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	items, _, err := client.ListPinsContext(ctx, channelID)
	if err != nil {
		return errResult(err)
	}
	type pin struct {
		Type    string `json:"type"`
		Channel string `json:"channel,omitempty"`
		Message string `json:"message,omitempty"`
		Created int64  `json:"created,omitempty"`
	}
	out := make([]pin, 0, len(items))
	for _, item := range items {
		p := pin{Type: item.Type}
		if item.Message != nil {
			p.Message = item.Message.Text
			p.Channel = item.Message.Channel
		}
		out = append(out, p)
	}
	return mcp.JSONResult(map[string]any{"count": len(out), "pins": out})
}

func scheduleMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	client, err := s.getClientForArgs(args)
	if err != nil {
		return errClientResult(err)
	}
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	text := r.Str("text")
	postAt := r.Str("post_at")
	threadTS := r.Str("thread_ts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}
	channel, scheduledID, err := client.ScheduleMessageContext(ctx, channelID, postAt, opts...)
	if err != nil {
		return errResult(err)
	}
	postAtInt, _ := strconv.ParseInt(postAt, 10, 64)
	return mcp.JSONResult(map[string]any{"status": "scheduled", "channel": channel, "scheduled_id": scheduledID, "post_at": postAtInt})
}
