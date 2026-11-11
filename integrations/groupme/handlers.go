package groupme

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

type handlerFunc func(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error)

// required returns an error naming the first empty value.
func required(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) == "" {
			return fmt.Errorf("groupme: %s is required", pairs[i])
		}
	}
	return nil
}

// setPositive adds key to q when n is set (> 0).
func setPositive(q url.Values, key string, n int) {
	if n > 0 {
		q.Set(key, strconv.Itoa(n))
	}
}

// setNonEmpty adds key to q when v is not empty.
func setNonEmpty(q url.Values, key, v string) {
	if v != "" {
		q.Set(key, v)
	}
}

// sourceGUID returns the caller's source_guid, or a new random one. GroupMe
// rejects a repeat source_guid within a minute, which guards against
// accidental double sends.
func sourceGUID(given string) (string, error) {
	if given != "" {
		return given, nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("groupme: cannot generate source_guid: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func get(ctx context.Context, g *groupme, path string, q url.Values) (*mcp.ToolResult, error) {
	data, err := g.doRequest(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if data == nil {
		return mcp.RawResult([]byte("null"))
	}
	return mcp.RawResult(data)
}

func getMe(ctx context.Context, g *groupme, _ map[string]any) (*mcp.ToolResult, error) {
	return get(ctx, g, "/users/me", nil)
}

func listGroups(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	page := r.OptInt("page", 0)
	perPage := r.OptInt("per_page", 0)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	// Member lists can be hundreds of entries per group; groupme_get_group
	// returns them for one group.
	q := url.Values{"omit": {"memberships"}}
	setPositive(q, "page", page)
	setPositive(q, "per_page", perPage)
	return get(ctx, g, "/groups", q)
}

func getGroup(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	groupID := r.Str("group_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required("group_id", groupID); err != nil {
		return mcp.ErrResult(err)
	}
	return get(ctx, g, "/groups/"+url.PathEscape(groupID), nil)
}

// emptyPage is what a message list returns for 304 Not Modified, which
// GroupMe sends when no messages match the cursor.
func emptyPage(key string) []byte {
	return []byte(`{"count":0,"` + key + `":[]}`)
}

func listGroupMessages(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	groupID := r.Str("group_id")
	beforeID := r.Str("before_id")
	sinceID := r.Str("since_id")
	afterID := r.Str("after_id")
	limit := r.OptInt("limit", 0)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required("group_id", groupID); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	setNonEmpty(q, "before_id", beforeID)
	setNonEmpty(q, "since_id", sinceID)
	setNonEmpty(q, "after_id", afterID)
	setPositive(q, "limit", limit)
	data, err := g.doRequest(ctx, http.MethodGet, "/groups/"+url.PathEscape(groupID)+"/messages", q, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if data == nil {
		return mcp.RawResult(emptyPage("messages"))
	}
	return mcp.RawResult(data)
}

func listChats(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	page := r.OptInt("page", 0)
	perPage := r.OptInt("per_page", 0)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	setPositive(q, "page", page)
	setPositive(q, "per_page", perPage)
	return get(ctx, g, "/chats", q)
}

func listDirectMessages(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	otherUserID := r.Str("other_user_id")
	beforeID := r.Str("before_id")
	sinceID := r.Str("since_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required("other_user_id", otherUserID); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{"other_user_id": {otherUserID}}
	setNonEmpty(q, "before_id", beforeID)
	setNonEmpty(q, "since_id", sinceID)
	data, err := g.doRequest(ctx, http.MethodGet, "/direct_messages", q, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if data == nil {
		return mcp.RawResult(emptyPage("direct_messages"))
	}
	return mcp.RawResult(data)
}

func listBots(ctx context.Context, g *groupme, _ map[string]any) (*mcp.ToolResult, error) {
	return get(ctx, g, "/bots", nil)
}

func post(ctx context.Context, g *groupme, path string, body any) (*mcp.ToolResult, error) {
	data, err := g.doRequest(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if data == nil {
		return mcp.JSONResult(map[string]string{"status": "sent"})
	}
	return mcp.RawResult(data)
}

func sendGroupMessage(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	groupID := r.Str("group_id")
	text := r.Str("text")
	guid := r.Str("source_guid")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required("group_id", groupID, "text", text); err != nil {
		return mcp.ErrResult(err)
	}
	guid, err := sourceGUID(guid)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"message": map[string]any{"source_guid": guid, "text": text}}
	return post(ctx, g, "/groups/"+url.PathEscape(groupID)+"/messages", body)
}

func sendDirectMessage(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recipientID := r.Str("recipient_id")
	text := r.Str("text")
	guid := r.Str("source_guid")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required("recipient_id", recipientID, "text", text); err != nil {
		return mcp.ErrResult(err)
	}
	guid, err := sourceGUID(guid)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"direct_message": map[string]any{"source_guid": guid, "recipient_id": recipientID, "text": text}}
	return post(ctx, g, "/direct_messages", body)
}

func postBotMessage(ctx context.Context, g *groupme, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	botID := r.Str("bot_id")
	text := r.Str("text")
	pictureURL := r.Str("picture_url")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if err := required("bot_id", botID, "text", text); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"bot_id": botID, "text": text}
	if pictureURL != "" {
		body["picture_url"] = pictureURL
	}
	return post(ctx, g, "/bots/post", body)
}
