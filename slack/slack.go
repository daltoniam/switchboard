package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

type slackIntegration struct {
	mu     sync.RWMutex
	client *slack.Client
	store  *tokenStore
	stopBg chan struct{}
}

func New() mcp.Integration {
	return &slackIntegration{}
}

func (s *slackIntegration) Name() string { return "slack" }

func (s *slackIntegration) Configure(creds mcp.Credentials) error {
	s.store = newTokenStore()

	// Seed the store from config credentials if present.
	if t := creds["token"]; t != "" {
		s.store.set(t, creds["cookie"])
	}

	// Try to load from the persistent token file (overrides config if fresher).
	s.store.loadFromFile()

	tok, cookie := s.store.get()
	if tok == "" {
		// Last resort: try Chrome extraction now.
		if extracted := extractFromChrome(); extracted != nil {
			s.store.set(extracted.token, extracted.cookie)
			s.store.saveToFile()
			tok = extracted.token
			cookie = extracted.cookie
		}
	}

	if tok == "" {
		return fmt.Errorf("slack: no token found — run with --web to configure, set SLACK_TOKEN/SLACK_COOKIE env vars, or open Slack in Chrome (macOS)")
	}

	s.buildClient(tok, cookie)

	// Start background refresh (every 4 hours).
	s.stopBg = make(chan struct{})
	go s.backgroundRefresh()

	return nil
}

// buildClient creates a new slack.Client with the cookie-injecting transport.
func (s *slackIntegration) buildClient(token, cookie string) {
	transport := &cookieTransport{
		cookie: cookie,
		inner:  http.DefaultTransport,
	}
	httpClient := &http.Client{Transport: transport}
	s.mu.Lock()
	s.client = slack.New(token, slack.OptionHTTPClient(httpClient))
	s.mu.Unlock()
}

// getClient returns the current slack.Client under read-lock.
func (s *slackIntegration) getClient() *slack.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *slackIntegration) Tools() []mcp.ToolDefinition { return tools }

func (s *slackIntegration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *slackIntegration) Healthy(ctx context.Context) bool {
	_, err := s.getClient().AuthTestContext(ctx)
	return err == nil
}

// backgroundRefresh checks token health every 4 hours and auto-refreshes from Chrome.
func (s *slackIntegration) backgroundRefresh() {
	ticker := time.NewTicker(4 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.tryRefresh()
		case <-s.stopBg:
			return
		}
	}
}

// tryRefresh attempts to extract fresh tokens from Chrome and rebuild the client.
func (s *slackIntegration) tryRefresh() bool {
	extracted := extractFromChrome()
	if extracted == nil {
		return false
	}
	s.store.set(extracted.token, extracted.cookie)
	s.store.saveToFile()
	s.buildClient(extracted.token, extracted.cookie)
	log.Println("slack: tokens refreshed from Chrome")
	return true
}

// --- cookie-injecting HTTP transport ---

// cookieTransport injects the Slack `d=` session cookie on every request.
// This is required for xoxc-* tokens which are tied to a browser session.
type cookieTransport struct {
	cookie string
	inner  http.RoundTripper
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.cookie != "" {
		req = req.Clone(req.Context())
		existing := req.Header.Get("Cookie")
		dCookie := "d=" + t.cookie
		if existing != "" {
			req.Header.Set("Cookie", existing+"; "+dCookie)
		} else {
			req.Header.Set("Cookie", dCookie)
		}
	}
	return t.inner.RoundTrip(req)
}

// --- handler function type and dispatch map ---

type handlerFunc func(ctx context.Context, s *slackIntegration, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[string]handlerFunc{
	// Token management
	"slack_token_status":    tokenStatus,
	"slack_refresh_tokens":  refreshTokens,

	// Conversations
	"slack_list_conversations":       listConversations,
	"slack_get_conversation_info":    getConversationInfo,
	"slack_conversations_history":    conversationsHistory,
	"slack_get_thread":               getThread,
	"slack_create_conversation":      createConversation,
	"slack_archive_conversation":     archiveConversation,
	"slack_invite_to_conversation":   inviteToConversation,
	"slack_kick_from_conversation":   kickFromConversation,
	"slack_set_conversation_topic":   setConversationTopic,
	"slack_set_conversation_purpose": setConversationPurpose,
	"slack_join_conversation":        joinConversation,
	"slack_leave_conversation":       leaveConversation,
	"slack_rename_conversation":      renameConversation,

	// Messages
	"slack_send_message":     sendMessage,
	"slack_update_message":   updateMessage,
	"slack_delete_message":   deleteMessage,
	"slack_search_messages":  searchMessages,
	"slack_add_reaction":     addReaction,
	"slack_remove_reaction":  removeReaction,
	"slack_get_reactions":    getReactions,
	"slack_add_pin":          addPin,
	"slack_remove_pin":       removePin,
	"slack_list_pins":        listPins,
	"slack_schedule_message": scheduleMessage,

	// Users
	"slack_list_users":        listUsers,
	"slack_get_user_info":     getUserInfo,
	"slack_get_user_presence": getUserPresence,
	"slack_list_user_groups":  listUserGroups,
	"slack_get_user_group":    getUserGroup,

	// Extras
	"slack_auth_test":       authTest,
	"slack_team_info":       teamInfo,
	"slack_upload_file":     uploadFile,
	"slack_list_files":      listFiles,
	"slack_delete_file":     deleteFile,
	"slack_list_emoji":      listEmoji,
	"slack_set_status":      setStatus,
	"slack_list_bookmarks":  listBookmarks,
	"slack_add_bookmark":    addBookmark,
	"slack_remove_bookmark": removeBookmark,
	"slack_add_reminder":    addReminder,
	"slack_list_reminders":  listReminders,
	"slack_delete_reminder": deleteReminder,
}

// --- helpers ---

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(err), nil
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) *mcp.ToolResult {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}
}

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
	case float64:
		return v != 0
	case string:
		return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
	}
	return false
}

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return nil
}

func optInt(args map[string]any, key string, def int) int {
	if _, ok := args[key]; !ok {
		return def
	}
	v := argInt(args, key)
	if v == 0 {
		return def
	}
	return v
}
