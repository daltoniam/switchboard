package instagram

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listConversations(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"platform": argStr(args, "platform"),
		"folder":   argStr(args, "folder"),
	})
	path := fmt.Sprintf("/%s/conversations?%s", ig.uid(args), q)
	data, err := ig.get(ctx, "%s", path)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getConversation(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	convID := argStr(args, "conversation_id")
	fields := argStr(args, "fields")
	if fields == "" {
		fields = "messages{id,message,from,to,created_time}"
	}
	data, err := ig.get(ctx, "/%s?fields=%s", convID, fields)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func sendMessage(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	recipientID := argStr(args, "recipient_id")
	message := argStr(args, "message")
	body := map[string]any{
		"recipient": map[string]string{"id": recipientID},
		"message":   map[string]string{"text": message},
	}
	path := fmt.Sprintf("/%s/messages", ig.uid(args))
	data, err := ig.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func sendMediaMessage(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error) {
	recipientID := argStr(args, "recipient_id")
	mediaURL := argStr(args, "media_url")
	mediaType := argStr(args, "media_type")
	body := map[string]any{
		"recipient": map[string]string{"id": recipientID},
		"message": map[string]any{
			"attachment": map[string]any{
				"type": mediaType,
				"payload": map[string]string{
					"url": mediaURL,
				},
			},
		},
	}
	path := fmt.Sprintf("/%s/messages", ig.uid(args))
	data, err := ig.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
