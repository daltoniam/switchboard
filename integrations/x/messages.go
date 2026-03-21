package x

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listDMEvents(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"dm_event.fields":  argStr(args, "dm_event_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/dm_events%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getDMConversation(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	participantID := argStr(args, "participant_id")
	q := queryEncode(map[string]string{
		"max_results":      argStr(args, "max_results"),
		"dm_event.fields":  argStr(args, "dm_event_fields"),
		"pagination_token": argStr(args, "pagination_token"),
	})
	data, err := t.get(ctx, "/dm_conversations/with/%s/dm_events%s", participantID, q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func sendDM(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	participantID := argStr(args, "participant_id")
	body := map[string]any{
		"text": argStr(args, "text"),
	}
	data, err := t.post(ctx, fmt.Sprintf("/dm_conversations/with/%s/messages", participantID), body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createDMConversation(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error) {
	participantIDs := strings.Split(argStr(args, "participant_ids"), ",")
	for i := range participantIDs {
		participantIDs[i] = strings.TrimSpace(participantIDs[i])
	}
	body := map[string]any{
		"conversation_type": "Group",
		"participant_ids":   participantIDs,
		"message": map[string]string{
			"text": argStr(args, "text"),
		},
	}
	data, err := t.post(ctx, "/dm_conversations", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
