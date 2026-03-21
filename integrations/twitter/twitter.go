package twitter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*twitter)(nil)
	_ mcp.FieldCompactionIntegration = (*twitter)(nil)
)

type twitter struct {
	bearerToken string
	client      *http.Client
	baseURL     string
	userID      string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &twitter{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.x.com/2",
	}
}

func (t *twitter) Name() string { return "twitter" }

func (t *twitter) Configure(_ context.Context, creds mcp.Credentials) error {
	t.bearerToken = creds["bearer_token"]
	if t.bearerToken == "" {
		return fmt.Errorf("twitter: bearer_token is required")
	}
	if v := creds["base_url"]; v != "" {
		t.baseURL = strings.TrimRight(v, "/")
	}

	me, err := t.fetchMe()
	if err != nil {
		return fmt.Errorf("twitter: failed to resolve authenticated user: %v", err)
	}
	t.userID = me
	return nil
}

func (t *twitter) fetchMe() (string, error) {
	req, err := http.NewRequest("GET", t.baseURL+"/users/me", nil)
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

func (t *twitter) Healthy(ctx context.Context) bool {
	_, err := t.get(ctx, "/users/me")
	return err == nil
}

func (t *twitter) Tools() []mcp.ToolDefinition {
	return tools
}

func (t *twitter) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (t *twitter) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, t, args)
}

// --- HTTP helpers ---

func (t *twitter) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
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
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("twitter API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("twitter API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (t *twitter) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return t.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (t *twitter) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return t.doRequest(ctx, "POST", path, body)
}

func (t *twitter) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return t.doRequest(ctx, "PUT", path, body)
}

func (t *twitter) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return t.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, t *twitter, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	if mcp.IsRetryable(err) {
		return nil, err
	}
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

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
func (t *twitter) me() string {
	return t.userID
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Tweets
	"twitter_get_tweet":            getTweet,
	"twitter_get_tweets":           getTweets,
	"twitter_create_tweet":         createTweet,
	"twitter_delete_tweet":         deleteTweet,
	"twitter_search_recent":        searchRecent,
	"twitter_search_all":           searchAll,
	"twitter_get_tweet_count":      getTweetCount,
	"twitter_get_quote_tweets":     getQuoteTweets,
	"twitter_hide_reply":           hideReply,
	"twitter_unhide_reply":         unhideReply,

	// Timelines
	"twitter_get_user_tweets":      getUserTweets,
	"twitter_get_user_mentions":    getUserMentions,
	"twitter_get_home_timeline":    getHomeTimeline,

	// Users
	"twitter_get_user":             getUser,
	"twitter_get_user_by_username": getUserByUsername,
	"twitter_get_users":            getUsers,
	"twitter_search_users":         searchUsers,
	"twitter_get_me":               getMe,

	// Follows
	"twitter_get_following":        getFollowing,
	"twitter_get_followers":        getFollowers,
	"twitter_follow_user":          followUser,
	"twitter_unfollow_user":        unfollowUser,

	// Blocks
	"twitter_get_blocked":          getBlocked,
	"twitter_block_user":           blockUser,
	"twitter_unblock_user":         unblockUser,

	// Mutes
	"twitter_get_muted":            getMuted,
	"twitter_mute_user":            muteUser,
	"twitter_unmute_user":          unmuteUser,

	// Likes
	"twitter_get_liking_users":     getLikingUsers,
	"twitter_get_liked_tweets":     getLikedTweets,
	"twitter_like_tweet":           likeTweet,
	"twitter_unlike_tweet":         unlikeTweet,

	// Retweets
	"twitter_get_retweeters":       getRetweeters,
	"twitter_retweet":              retweet,
	"twitter_unretweet":            unretweet,

	// Bookmarks
	"twitter_get_bookmarks":        getBookmarks,
	"twitter_bookmark_tweet":       bookmarkTweet,
	"twitter_remove_bookmark":      removeBookmark,

	// Lists
	"twitter_get_list":             getList,
	"twitter_get_owned_lists":      getOwnedLists,
	"twitter_create_list":          createList,
	"twitter_update_list":          updateList,
	"twitter_delete_list":          deleteList,
	"twitter_get_list_tweets":      getListTweets,
	"twitter_get_list_members":     getListMembers,
	"twitter_add_list_member":      addListMember,
	"twitter_remove_list_member":   removeListMember,
	"twitter_get_list_followers":   getListFollowers,
	"twitter_follow_list":          followList,
	"twitter_unfollow_list":        unfollowList,
	"twitter_get_pinned_lists":     getPinnedLists,
	"twitter_pin_list":             pinList,
	"twitter_unpin_list":           unpinList,

	// Direct Messages
	"twitter_list_dm_events":       listDMEvents,
	"twitter_get_dm_conversation":  getDMConversation,
	"twitter_send_dm":              sendDM,
	"twitter_create_dm_conversation": createDMConversation,

	// Spaces
	"twitter_get_space":            getSpace,
	"twitter_search_spaces":        searchSpaces,

	// Usage
	"twitter_get_usage":            getUsage,
}
