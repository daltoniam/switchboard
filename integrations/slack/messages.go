package slack

import (
	"context"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

var _ *mcp.ToolResult // type anchor

func sendMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	opts := []slack.MsgOption{slack.MsgOptionText(argStr(args, "text"), false)}
	if ts := argStr(args, "thread_ts"); ts != "" {
		opts = append(opts, slack.MsgOptionTS(ts))
	}

	channel, ts, err := s.getClient().PostMessageContext(ctx, argStr(args, "channel_id"), opts...)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "sent", "channel": channel, "ts": ts})
}

func updateMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	opts := []slack.MsgOption{slack.MsgOptionText(argStr(args, "text"), false)}
	channel, ts, _, err := s.getClient().UpdateMessageContext(ctx, argStr(args, "channel_id"), argStr(args, "ts"), opts...)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "updated", "channel": channel, "ts": ts})
}

func deleteMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	channel, ts, err := s.getClient().DeleteMessageContext(ctx, argStr(args, "channel_id"), argStr(args, "ts"))
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "deleted", "channel": channel, "ts": ts})
}

func searchMessages(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	params := slack.SearchParameters{
		Sort:          "timestamp",
		SortDirection: "desc",
		Count:         optInt(args, "count", 20),
	}
	result, err := s.getClient().SearchMessagesContext(ctx, argStr(args, "query"), params)
	if err != nil {
		return errResult(err), nil
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
		matches = append(matches, match{
			TS:        m.Timestamp,
			Channel:   m.Channel.Name,
			ChannelID: m.Channel.ID,
			User:      m.User,
			Text:      m.Text,
			Permalink: m.Permalink,
		})
	}
	return jsonResult(map[string]any{
		"query":   argStr(args, "query"),
		"total":   result.Total,
		"matches": matches,
	})
}

func addReaction(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ref := slack.ItemRef{Channel: argStr(args, "channel_id"), Timestamp: argStr(args, "ts")}
	err := s.getClient().AddReactionContext(ctx, argStr(args, "emoji"), ref)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "added", "emoji": argStr(args, "emoji")})
}

func removeReaction(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ref := slack.ItemRef{Channel: argStr(args, "channel_id"), Timestamp: argStr(args, "ts")}
	err := s.getClient().RemoveReactionContext(ctx, argStr(args, "emoji"), ref)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "removed", "emoji": argStr(args, "emoji")})
}

func getReactions(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ref := slack.ItemRef{Channel: argStr(args, "channel_id"), Timestamp: argStr(args, "ts")}
	reactedItem, err := s.getClient().GetReactionsContext(ctx, ref, slack.GetReactionsParameters{})
	if err != nil {
		return errResult(err), nil
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
	return jsonResult(out)
}

func addPin(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ref := slack.ItemRef{Channel: argStr(args, "channel_id"), Timestamp: argStr(args, "ts")}
	err := s.getClient().AddPinContext(ctx, argStr(args, "channel_id"), ref)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "pinned"})
}

func removePin(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	ref := slack.ItemRef{Channel: argStr(args, "channel_id"), Timestamp: argStr(args, "ts")}
	err := s.getClient().RemovePinContext(ctx, argStr(args, "channel_id"), ref)
	if err != nil {
		return errResult(err), nil
	}
	return jsonResult(map[string]any{"status": "unpinned"})
}

func listPins(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	items, _, err := s.getClient().ListPinsContext(ctx, argStr(args, "channel_id"))
	if err != nil {
		return errResult(err), nil
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
	return jsonResult(map[string]any{"count": len(out), "pins": out})
}

func scheduleMessage(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error) {
	postAt := argStr(args, "post_at")
	opts := []slack.MsgOption{slack.MsgOptionText(argStr(args, "text"), false)}
	if ts := argStr(args, "thread_ts"); ts != "" {
		opts = append(opts, slack.MsgOptionTS(ts))
	}

	channel, scheduledID, err := s.getClient().ScheduleMessageContext(ctx, argStr(args, "channel_id"), postAt, opts...)
	if err != nil {
		return errResult(err), nil
	}

	postAtInt, _ := strconv.ParseInt(postAt, 10, 64)
	return jsonResult(map[string]any{
		"status":       "scheduled",
		"channel":      channel,
		"scheduled_id": scheduledID,
		"post_at":      postAtInt,
	})
}
