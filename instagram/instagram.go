package instagram

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

type instagram struct {
	accessToken string
	userID      string
	client      *http.Client
	baseURL     string
	apiVersion  string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &instagram{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://graph.instagram.com",
		apiVersion: "v21.0",
	}
}

func (ig *instagram) Name() string { return "instagram" }

func (ig *instagram) Configure(creds mcp.Credentials) error {
	ig.accessToken = creds["access_token"]
	if ig.accessToken == "" {
		return fmt.Errorf("instagram: access_token is required")
	}
	ig.userID = creds["user_id"]
	if ig.userID == "" {
		return fmt.Errorf("instagram: user_id is required")
	}
	if v := creds["api_version"]; v != "" {
		ig.apiVersion = v
	}
	return nil
}

func (ig *instagram) Healthy(ctx context.Context) bool {
	if ig.accessToken == "" {
		return false
	}
	_, err := ig.get(ctx, "/me?fields=id,username")
	return err == nil
}

func (ig *instagram) Tools() []mcp.ToolDefinition {
	return tools
}

func (ig *instagram) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, ig, args)
}

// --- HTTP helpers ---

func (ig *instagram) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := ig.baseURL + "/" + ig.apiVersion + path
	if strings.Contains(path, "?") {
		fullURL += "&access_token=" + ig.accessToken
	} else {
		fullURL += "?access_token=" + ig.accessToken
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ig.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("instagram API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (ig *instagram) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return ig.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (ig *instagram) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return ig.doRequest(ctx, "POST", path, body)
}

func (ig *instagram) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return ig.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, ig *instagram, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
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
	return "&" + vals.Encode()
}

// uid returns the user ID from args, falling back to the configured default.
func (ig *instagram) uid(args map[string]any) string {
	if v := argStr(args, "user_id"); v != "" {
		return v
	}
	return ig.userID
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Profiles
	"instagram_get_profile":            getProfile,
	"instagram_discover_user":          discoverUser,
	"instagram_get_recently_searched":  getRecentlySearched,

	// Media
	"instagram_list_media":             listMedia,
	"instagram_get_media":              getMedia,
	"instagram_list_stories":           listStories,
	"instagram_get_story":              getStory,
	"instagram_list_media_children":    listMediaChildren,

	// Messaging
	"instagram_list_conversations":     listConversations,
	"instagram_get_conversation":       getConversation,
	"instagram_send_message":           sendMessage,
	"instagram_send_media_message":     sendMediaMessage,

	// Comments
	"instagram_list_comments":          listComments,
	"instagram_get_comment":            getComment,
	"instagram_reply_to_comment":       replyToComment,
	"instagram_list_comment_replies":   listCommentReplies,
	"instagram_hide_comment":           hideComment,
	"instagram_delete_comment":         deleteComment,
	"instagram_get_mentioned_comment":  getMentionedComment,
	"instagram_get_mentioned_media":    getMentionedMedia,

	// Insights
	"instagram_get_media_insights":     getMediaInsights,
	"instagram_get_account_insights":   getAccountInsights,

	// Hashtags
	"instagram_search_hashtag":         searchHashtag,
	"instagram_get_hashtag_recent":     getHashtagRecent,
	"instagram_get_hashtag_top":        getHashtagTop,

	// Publishing
	"instagram_create_media_container": createMediaContainer,
	"instagram_publish_media":          publishMedia,
	"instagram_get_publish_status":     getPublishStatus,
}
