package x

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listDMEvents(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"dm_event.fields":  r.Str("dm_event_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/dm_events%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDMConversation(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	participantID := r.Str("participant_id")
	q := queryEncode(map[string]string{
		"max_results":      r.Str("max_results"),
		"dm_event.fields":  r.Str("dm_event_fields"),
		"pagination_token": r.Str("pagination_token"),
	})
	data, err := t.get(ctx, "/dm_conversations/with/%s/dm_events%s", participantID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func sendDM(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	participantID := r.Str("participant_id")
	body := map[string]any{
		"text": r.Str("text"),
	}
	data, err := t.post(ctx, fmt.Sprintf("/dm_conversations/with/%s/messages", participantID), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDMConversation(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	participantIDs := strings.Split(r.Str("participant_ids"), ",")
	for i := range participantIDs {
		participantIDs[i] = strings.TrimSpace(participantIDs[i])
	}
	body := map[string]any{
		"conversation_type": "Group",
		"participant_ids":   participantIDs,
		"message": map[string]string{
			"text": r.Str("text"),
		},
	}
	data, err := t.post(ctx, "/dm_conversations", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
