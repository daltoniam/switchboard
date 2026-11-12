package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

type handlerFunc func(ctx context.Context, d *discord, args map[string]any) (*mcp.ToolResult, error)

// mentionsUsersOnly lets user mentions notify but never @everyone, @here or
// roles, so a model-written message cannot ping a whole server.
var mentionsUsersOnly = map[string]any{"parse": []string{"users"}}

// requiredID checks that a required ID is present and is a Discord
// snowflake (digits only), which also keeps it safe inside a URL path.
func requiredID(name, v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("discord: %s is required", name)
	}
	return optionalID(name, v)
}

func optionalID(name, v string) error {
	if v == "" {
		return nil
	}
	if _, err := strconv.ParseUint(v, 10, 64); err != nil {
		return fmt.Errorf("discord: %s must be a numeric Discord ID", name)
	}
	return nil
}

func requiredText(name, v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("discord: %s is required", name)
	}
	return nil
}

// firstErr returns the first non-nil error.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func result(data json.RawMessage, err error) (*mcp.ToolResult, error) {
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMe(ctx context.Context, d *discord, _ map[string]any) (*mcp.ToolResult, error) {
	return result(d.doRequest(ctx, http.MethodGet, "/users/@me", nil, nil))
}

func listGuilds(ctx context.Context, d *discord, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	limit := r.OptInt("limit", 0)
	before := r.Str("before")
	after := r.Str("after")
	if err := firstErr(r.Err(), optionalID("before", before), optionalID("after", after)); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if before != "" {
		q.Set("before", before)
	}
	if after != "" {
		q.Set("after", after)
	}
	return result(d.doRequest(ctx, http.MethodGet, "/users/@me/guilds", q, nil))
}

func listChannels(ctx context.Context, d *discord, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	guildID := r.Str("guild_id")
	if err := firstErr(r.Err(), requiredID("guild_id", guildID)); err != nil {
		return mcp.ErrResult(err)
	}
	return result(d.doRequest(ctx, http.MethodGet, "/guilds/"+guildID+"/channels", nil, nil))
}

func listChannelMessages(ctx context.Context, d *discord, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	limit := r.OptInt("limit", 0)
	before := r.Str("before")
	after := r.Str("after")
	around := r.Str("around")
	if err := firstErr(r.Err(), requiredID("channel_id", channelID),
		optionalID("before", before), optionalID("after", after), optionalID("around", around)); err != nil {
		return mcp.ErrResult(err)
	}
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	for key, v := range map[string]string{"before": before, "after": after, "around": around} {
		if v != "" {
			q.Set(key, v)
		}
	}
	return result(d.doRequest(ctx, http.MethodGet, "/channels/"+channelID+"/messages", q, nil))
}

func getUser(ctx context.Context, d *discord, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	userID := r.Str("user_id")
	if err := firstErr(r.Err(), requiredID("user_id", userID)); err != nil {
		return mcp.ErrResult(err)
	}
	return result(d.doRequest(ctx, http.MethodGet, "/users/"+userID, nil, nil))
}

func sendChannelMessage(ctx context.Context, d *discord, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	channelID := r.Str("channel_id")
	content := r.Str("content")
	replyTo := r.Str("reply_to_message_id")
	if err := firstErr(r.Err(), requiredID("channel_id", channelID), requiredText("content", content),
		optionalID("reply_to_message_id", replyTo)); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"content": content, "allowed_mentions": mentionsUsersOnly}
	if replyTo != "" {
		body["message_reference"] = map[string]any{"message_id": replyTo, "fail_if_not_exists": false}
	}
	return result(d.doRequest(ctx, http.MethodPost, "/channels/"+channelID+"/messages", nil, body))
}

func sendDirectMessage(ctx context.Context, d *discord, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	recipientID := r.Str("recipient_id")
	content := r.Str("content")
	if err := firstErr(r.Err(), requiredID("recipient_id", recipientID), requiredText("content", content)); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := d.doRequest(ctx, http.MethodPost, "/users/@me/channels", nil, map[string]any{"recipient_id": recipientID})
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("discord: cannot open DM channel: %w", err))
	}
	var channel struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &channel); err != nil || optionalID("channel id", channel.ID) != nil || channel.ID == "" {
		return mcp.ErrResult(fmt.Errorf("discord: DM channel response has no valid id"))
	}
	body := map[string]any{"content": content, "allowed_mentions": mentionsUsersOnly}
	return result(d.doRequest(ctx, http.MethodPost, "/channels/"+channel.ID+"/messages", nil, body))
}
