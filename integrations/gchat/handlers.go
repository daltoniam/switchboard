package gchat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// normalizeSpaceID accepts either a bare space ID ("AAQAtuk0o-A") or the
// full resource name ("spaces/AAQAtuk0o-A") and returns the bare ID. The
// Chat API requires resource-name form in URLs; handlers escape and prepend
// the "spaces/" prefix themselves so callers can pass either shape.
func normalizeSpaceID(s string) string {
	return strings.TrimPrefix(s, "spaces/")
}

// parseCardsV2 accepts either a raw JSON string ("[{...}]") or an
// already-decoded slice/array and returns the value to attach to the
// request body as cardsV2.
func parseCardsV2(v any) (any, error) {
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return nil, nil
		}
		var decoded any
		if err := json.Unmarshal([]byte(x), &decoded); err != nil {
			return nil, fmt.Errorf("cards_v2 is not valid JSON: %w", err)
		}
		return decoded, nil
	case nil:
		return nil, nil
	default:
		return x, nil
	}
}

// ── Space resource ──────────────────────────────────────────────────

func listSpaces(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	params := url.Values{}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	if filter != "" {
		params.Set("filter", filter)
	}

	path := "/spaces"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getSpace(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spaceID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_get_space: space_id is required"))
	}

	path := "/spaces/" + url.PathEscape(normalizeSpaceID(spaceID))
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Message resource ────────────────────────────────────────────────

func listMessages(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	filter := r.Str("filter")
	orderBy := r.Str("order_by")
	showDeleted := r.Bool("show_deleted")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spaceID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_list_messages: space_id is required"))
	}

	params := url.Values{}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	if filter != "" {
		params.Set("filter", filter)
	}
	if orderBy != "" {
		params.Set("orderBy", orderBy)
	}
	if showDeleted {
		params.Set("showDeleted", "true")
	}

	path := "/spaces/" + url.PathEscape(normalizeSpaceID(spaceID)) + "/messages"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMessage(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	messageID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spaceID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_get_message: space_id is required"))
	}
	if messageID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_get_message: message_id is required"))
	}

	path := "/spaces/" + url.PathEscape(normalizeSpaceID(spaceID)) +
		"/messages/" + url.PathEscape(messageID)
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createMessage(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	text := r.Str("text")
	threadKey := r.Str("thread_key")
	messageReplyOption := r.Str("message_reply_option")
	messageID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spaceID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_create_message: space_id is required"))
	}

	cardsV2, err := parseCardsV2(args["cards_v2"])
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("gchat_create_message: %w", err))
	}
	if text == "" && cardsV2 == nil {
		return mcp.ErrResult(fmt.Errorf("gchat_create_message: either text or cards_v2 is required"))
	}

	body := map[string]any{}
	if text != "" {
		body["text"] = text
	}
	if cardsV2 != nil {
		body["cardsV2"] = cardsV2
	}
	if threadKey != "" {
		body["thread"] = map[string]any{"threadKey": threadKey}
	}

	params := url.Values{}
	if messageReplyOption != "" {
		params.Set("messageReplyOption", messageReplyOption)
	}
	if messageID != "" {
		params.Set("messageId", messageID)
	}
	path := "/spaces/" + url.PathEscape(normalizeSpaceID(spaceID)) + "/messages"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateMessage(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	messageID := r.Str("message_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spaceID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_update_message: space_id is required"))
	}
	if messageID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_update_message: message_id is required"))
	}

	body := map[string]any{}
	var maskFields []string

	if v, ok := args["text"]; ok {
		body["text"] = v
		maskFields = append(maskFields, "text")
	}
	if _, ok := args["cards_v2"]; ok {
		cardsV2, err := parseCardsV2(args["cards_v2"])
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("gchat_update_message: %w", err))
		}
		body["cardsV2"] = cardsV2
		maskFields = append(maskFields, "cardsV2")
	}
	if len(maskFields) == 0 {
		return mcp.ErrResult(fmt.Errorf("gchat_update_message: at least one of text or cards_v2 is required"))
	}

	params := url.Values{}
	params.Set("updateMask", strings.Join(maskFields, ","))
	path := "/spaces/" + url.PathEscape(normalizeSpaceID(spaceID)) +
		"/messages/" + url.PathEscape(messageID) + "?" + params.Encode()

	data, err := g.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteMessage(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	messageID := r.Str("message_id")
	force := r.Bool("force")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spaceID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_delete_message: space_id is required"))
	}
	if messageID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_delete_message: message_id is required"))
	}

	path := "/spaces/" + url.PathEscape(normalizeSpaceID(spaceID)) +
		"/messages/" + url.PathEscape(messageID)
	if force {
		path += "?force=true"
	}
	data, err := g.delete(ctx, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Membership resource ─────────────────────────────────────────────

func listMembers(ctx context.Context, g *gchat, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	spaceID := r.Str("space_id")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	showInvited := r.Bool("show_invited")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if spaceID == "" {
		return mcp.ErrResult(fmt.Errorf("gchat_list_members: space_id is required"))
	}

	params := url.Values{}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	if showInvited {
		params.Set("showInvited", "true")
	}
	if filter != "" {
		params.Set("filter", filter)
	}

	path := "/spaces/" + url.PathEscape(normalizeSpaceID(spaceID)) + "/members"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
