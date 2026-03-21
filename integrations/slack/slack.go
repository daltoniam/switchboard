package slack

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*slackIntegration)(nil)
	_ mcp.FieldCompactionIntegration = (*slackIntegration)(nil)
)

type slackIntegration struct {
	mu      sync.RWMutex
	clients map[string]*slack.Client // keyed by team_id
	store   *tokenStore
	stopBg  chan struct{}
}

func New() mcp.Integration {
	return &slackIntegration{}
}

func (s *slackIntegration) Name() string { return "slack" }

func (s *slackIntegration) Configure(_ context.Context, creds mcp.Credentials) error {
	s.store = newTokenStore()
	s.mu.Lock()
	s.clients = make(map[string]*slack.Client)
	s.mu.Unlock()

	if t := creds["token"]; t != "" {
		teamID := creds["team_id"]
		if teamID == "" {
			teamID = "_config"
		}
		s.store.setWorkspace(&workspace{
			TeamID: teamID,
			Token:  t,
			Cookie: creds["cookie"],
			Source: "config",
		})
		if creds["team_id"] != "" {
			s.store.setDefault(creds["team_id"])
		}
	}

	s.store.loadFromFile()

	if len(s.store.allWorkspaces()) == 0 {
		wss, _ := listWorkspacesFromChrome()
		for _, ws := range wss {
			s.store.setWorkspace(&workspace{
				TeamID:   ws.TeamID,
				TeamName: ws.Name,
				Source:   "chrome",
			})
		}
	}

	if len(s.store.allWorkspaces()) == 0 {
		return fmt.Errorf("slack: no token found — run with --web to configure, set SLACK_TOKEN/SLACK_COOKIE env vars, or open Slack in Chrome (macOS)")
	}

	s.buildAllClients()
	s.resolveWorkspaceIdentities()

	if tid := creds["team_id"]; tid != "" {
		s.store.setDefault(tid)
	}

	_ = s.store.saveToFile()

	s.stopBg = make(chan struct{})
	go s.backgroundRefresh()

	return nil
}

func (s *slackIntegration) buildAllClients() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ws := range s.store.allWorkspaces() {
		transport := &cookieTransport{cookie: ws.Cookie, inner: http.DefaultTransport}
		s.clients[ws.TeamID] = slack.New(ws.Token, slack.OptionHTTPClient(&http.Client{Transport: transport}))
	}
}

func (s *slackIntegration) buildClientForWorkspace(ws *workspace) {
	transport := &cookieTransport{cookie: ws.Cookie, inner: http.DefaultTransport}
	s.mu.Lock()
	s.clients[ws.TeamID] = slack.New(ws.Token, slack.OptionHTTPClient(&http.Client{Transport: transport}))
	s.mu.Unlock()
}

func (s *slackIntegration) resolveWorkspaceIdentities() {
	for _, ws := range s.store.allWorkspaces() {
		client := s.getClientForTeam(ws.TeamID)
		if client == nil {
			continue
		}
		resp, err := client.AuthTest()
		if err != nil {
			log.Printf("slack: auth test failed for workspace %s: %v", ws.TeamID, err)
			continue
		}
		if ws.TeamID == resp.TeamID {
			if ws.TeamName == "" {
				ws.TeamName = resp.Team
				s.store.setWorkspace(ws)
			}
			continue
		}
		wasDefault := s.store.defaultID() == ws.TeamID
		s.mu.Lock()
		delete(s.clients, ws.TeamID)
		s.mu.Unlock()
		s.store.removeWorkspace(ws.TeamID)
		if existing := s.store.getWorkspace(resp.TeamID); existing != nil {
			if existing.TeamName == "" {
				existing.TeamName = resp.Team
				s.store.setWorkspace(existing)
			}
			if wasDefault {
				s.store.setDefault(resp.TeamID)
			}
			continue
		}
		ws.TeamID = resp.TeamID
		ws.TeamName = resp.Team
		s.store.setWorkspace(ws)
		s.buildClientForWorkspace(ws)
		if wasDefault {
			s.store.setDefault(resp.TeamID)
		}
	}
}

func (s *slackIntegration) getClientForTeam(teamID string) *slack.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if teamID == "" {
		teamID = s.store.defaultID()
	}
	return s.clients[teamID]
}

func (s *slackIntegration) getClientForArgs(args map[string]any) (*slack.Client, error) {
	teamID, _ := mcp.ArgStr(args, "team_id")
	client := s.getClientForTeam(teamID)
	if client == nil {
		if teamID != "" {
			return nil, fmt.Errorf("unknown workspace: %s — use slack_list_workspaces to see available workspaces", teamID)
		}
		return nil, fmt.Errorf("no slack workspace configured")
	}
	return client, nil
}

func (s *slackIntegration) getClient() *slack.Client {
	return s.getClientForTeam("")
}

func (s *slackIntegration) Tools() []mcp.ToolDefinition { return tools }

func (s *slackIntegration) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *slackIntegration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, s, args)
}

func (s *slackIntegration) Healthy(ctx context.Context) bool {
	client := s.getClient()
	if client == nil {
		return false
	}
	_, err := client.AuthTestContext(ctx)
	return err == nil
}

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

func (s *slackIntegration) tryRefresh() bool {
	allOk := true
	for _, ws := range s.store.allWorkspaces() {
		if strings.HasPrefix(ws.Token, "xoxp-") {
			continue
		}
		if !s.tryRefreshWorkspace(ws.TeamID) {
			allOk = false
		}
	}
	return allOk
}

func (s *slackIntegration) tryRefreshWorkspace(teamID string) bool {
	if s.tryRefreshViaCookieForTeam(teamID) {
		return true
	}
	extracted := extractFromChrome(teamID)
	if extracted == nil {
		return false
	}
	s.store.updateTokens(teamID, extracted.token, extracted.cookie)
	ws := s.store.getWorkspace(teamID)
	if ws != nil {
		s.buildClientForWorkspace(ws)
	}
	_ = s.store.saveToFile()
	log.Printf("slack: tokens refreshed from Chrome for %s", teamID)
	return true
}

// --- cookie-injecting HTTP transport ---

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
	"slack_token_status":             tokenStatus,
	"slack_refresh_tokens":           refreshTokens,
	"slack_list_workspaces":          listWorkspaces,
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
	"slack_send_message":             sendMessage,
	"slack_update_message":           updateMessage,
	"slack_delete_message":           deleteMessage,
	"slack_search_messages":          searchMessages,
	"slack_add_reaction":             addReaction,
	"slack_remove_reaction":          removeReaction,
	"slack_get_reactions":            getReactions,
	"slack_add_pin":                  addPin,
	"slack_remove_pin":               removePin,
	"slack_list_pins":                listPins,
	"slack_schedule_message":         scheduleMessage,
	"slack_list_users":               listUsers,
	"slack_get_user_info":            getUserInfo,
	"slack_get_user_presence":        getUserPresence,
	"slack_list_user_groups":         listUserGroups,
	"slack_get_user_group":           getUserGroup,
	"slack_auth_test":                authTest,
	"slack_team_info":                teamInfo,
	"slack_upload_file":              uploadFile,
	"slack_list_files":               listFiles,
	"slack_delete_file":              deleteFile,
	"slack_list_emoji":               listEmoji,
	"slack_set_status":               setStatus,
	"slack_list_bookmarks":           listBookmarks,
	"slack_add_bookmark":             addBookmark,
	"slack_remove_bookmark":          removeBookmark,
	"slack_add_reminder":             addReminder,
	"slack_list_reminders":           listReminders,
	"slack_delete_reminder":          deleteReminder,
}

// --- helpers ---

func wrapRetryable(err error) error {
	if err == nil {
		return nil
	}
	var rle *slack.RateLimitedError
	if errors.As(err, &rle) {
		return &mcp.RetryableError{StatusCode: 429, Err: err, RetryAfter: rle.RetryAfter}
	}
	return err
}

func errResult(err error) (*mcp.ToolResult, error) {
	return mcp.ErrResult(wrapRetryable(err))
}
