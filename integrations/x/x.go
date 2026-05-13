package x

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compactyaml"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compactyaml.MustLoadWithOverlay("x", compactYAML, compactyaml.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*xClient)(nil)
	_ mcp.FieldCompactionIntegration = (*xClient)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*xClient)(nil)
)

type xClient struct {
	bearerToken string
	client      *http.Client
	baseURL     string
	userID      string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &xClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.x.com/2",
	}
}

func (t *xClient) Name() string { return "x" }

func (t *xClient) Configure(ctx context.Context, creds mcp.Credentials) error {
	t.bearerToken = creds["bearer_token"]
	if t.bearerToken == "" {
		return fmt.Errorf("x: bearer_token is required")
	}
	if v := creds["base_url"]; v != "" {
		t.baseURL = strings.TrimRight(v, "/")
	}

	me, err := t.fetchMe(ctx)
	if err != nil {
		return fmt.Errorf("x: failed to resolve authenticated user: %v", err)
	}
	t.userID = me
	return nil
}

func (t *xClient) fetchMe(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/users/me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+t.bearerToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse user response: %w", err)
	}
	if result.Data.ID == "" {
		return "", fmt.Errorf("no user ID in response")
	}
	return result.Data.ID, nil
}

func (t *xClient) Healthy(ctx context.Context) bool {
	_, err := t.get(ctx, "/users/me")
	return err == nil
}

func (t *xClient) Tools() []mcp.ToolDefinition {
	return tools
}

func (t *xClient) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (t *xClient) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (t *xClient) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, t, args)
}

// --- HTTP helpers ---

func (t *xClient) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.bearerToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("x API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("x API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (t *xClient) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return t.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (t *xClient) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return t.doRequest(ctx, "POST", path, body)
}

func (t *xClient) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return t.doRequest(ctx, "PUT", path, body)
}

func (t *xClient) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return t.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, t *xClient, args map[string]any) (*mcp.ToolResult, error)

func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

// me returns the authenticated user ID.
func (t *xClient) me() string {
	return t.userID
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Tweets
	"x_get_tweet":        getTweet,
	"x_get_tweets":       getTweets,
	"x_create_tweet":     createTweet,
	"x_delete_tweet":     deleteTweet,
	"x_search_recent":    searchRecent,
	"x_search_all":       searchAll,
	"x_get_tweet_count":  getTweetCount,
	"x_get_quote_tweets": getQuoteTweets,
	"x_hide_reply":       hideReply,
	"x_unhide_reply":     unhideReply,

	// Timelines
	"x_get_user_tweets":   getUserTweets,
	"x_get_user_mentions": getUserMentions,
	"x_get_home_timeline": getHomeTimeline,

	// Users
	"x_get_user":             getUser,
	"x_get_user_by_username": getUserByUsername,
	"x_get_users":            getUsers,
	"x_search_users":         searchUsers,
	"x_get_me":               getMe,

	// Follows
	"x_get_following": getFollowing,
	"x_get_followers": getFollowers,
	"x_follow_user":   followUser,
	"x_unfollow_user": unfollowUser,

	// Blocks
	"x_get_blocked":  getBlocked,
	"x_block_user":   blockUser,
	"x_unblock_user": unblockUser,

	// Mutes
	"x_get_muted":   getMuted,
	"x_mute_user":   muteUser,
	"x_unmute_user": unmuteUser,

	// Likes
	"x_get_liking_users": getLikingUsers,
	"x_get_liked_tweets": getLikedTweets,
	"x_like_tweet":       likeTweet,
	"x_unlike_tweet":     unlikeTweet,

	// Retweets
	"x_get_retweeters": getRetweeters,
	"x_retweet":        retweet,
	"x_unretweet":      unretweet,

	// Bookmarks
	"x_get_bookmarks":   getBookmarks,
	"x_bookmark_tweet":  bookmarkTweet,
	"x_remove_bookmark": removeBookmark,

	// Lists
	"x_get_list":           getList,
	"x_get_owned_lists":    getOwnedLists,
	"x_create_list":        createList,
	"x_update_list":        updateList,
	"x_delete_list":        deleteList,
	"x_get_list_tweets":    getListTweets,
	"x_get_list_members":   getListMembers,
	"x_add_list_member":    addListMember,
	"x_remove_list_member": removeListMember,
	"x_get_list_followers": getListFollowers,
	"x_follow_list":        followList,
	"x_unfollow_list":      unfollowList,
	"x_get_pinned_lists":   getPinnedLists,
	"x_pin_list":           pinList,
	"x_unpin_list":         unpinList,

	// Direct Messages
	"x_list_dm_events":         listDMEvents,
	"x_get_dm_conversation":    getDMConversation,
	"x_send_dm":                sendDM,
	"x_create_dm_conversation": createDMConversation,

	// Spaces
	"x_get_space":     getSpace,
	"x_search_spaces": searchSpaces,

	// Usage
	"x_get_usage": getUsage,
}
