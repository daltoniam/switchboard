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

func (s *slackIntegration) Configure(ctx context.Context, creds mcp.Credentials) error {
	s.store = newTokenStore()
	s.mu.Lock()
	s.clients = make(map[string]*slack.Client)
	s.mu.Unlock()

	configToken := creds["token"]
	if configToken != "" {
		teamID := creds["team_id"]
		if teamID == "" {
			teamID = "_config"
		}
		s.store.setWorkspace(&workspace{
			TeamID: teamID,
			Token:  configToken,
			Cookie: creds["cookie"],
			Source: "config",
		})
		if creds["team_id"] != "" {
			s.store.setDefault(creds["team_id"])
		}
	}

	// When a token was provided via config (e.g. OAuth in hosted environments),
	// skip local file and Chrome extraction — the config token is authoritative.
	if configToken == "" {
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
	}

	if len(s.store.allWorkspaces()) == 0 {
		return fmt.Errorf("slack: no token found — run with --web to configure, set SLACK_TOKEN/SLACK_COOKIE env vars, or open Slack in Chrome (macOS)")
	}

	s.buildAllClients()
	s.resolveWorkspaceIdentities(ctx)

	if tid := creds["team_id"]; tid != "" {
		s.store.setDefault(tid)
	}

	// Only persist and run background refresh for locally-sourced tokens.
	// Config-provided tokens are managed externally and don't need local refresh.
	if configToken == "" {
		_ = s.store.saveToFile()

		if s.stopBg != nil {
			close(s.stopBg)
		}
		s.stopBg = make(chan struct{})
		go s.backgroundRefresh()
	} else if s.stopBg != nil {
		close(s.stopBg)
		s.stopBg = nil
	}

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

func (s *slackIntegration) resolveWorkspaceIdentities(ctx context.Context) {
	for _, ws := range s.store.allWorkspaces() {
		client := s.getClientForTeam(ws.TeamID)
		if client == nil {
			continue
		}
		resp, err := client.AuthTestContext(ctx)
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

func (s *slackIntegration) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (s *slackIntegration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("slack_token_status"):             tokenStatus,
	mcp.ToolName("slack_refresh_tokens"):           refreshTokens,
	mcp.ToolName("slack_list_workspaces"):          listWorkspaces,
	mcp.ToolName("slack_list_conversations"):       listConversations,
	mcp.ToolName("slack_get_conversation_info"):    getConversationInfo,
	mcp.ToolName("slack_conversations_history"):    conversationsHistory,
	mcp.ToolName("slack_get_thread"):               getThread,
	mcp.ToolName("slack_create_conversation"):      createConversation,
	mcp.ToolName("slack_archive_conversation"):     archiveConversation,
	mcp.ToolName("slack_invite_to_conversation"):   inviteToConversation,
	mcp.ToolName("slack_kick_from_conversation"):   kickFromConversation,
	mcp.ToolName("slack_set_conversation_topic"):   setConversationTopic,
	mcp.ToolName("slack_set_conversation_purpose"): setConversationPurpose,
	mcp.ToolName("slack_join_conversation"):        joinConversation,
	mcp.ToolName("slack_leave_conversation"):       leaveConversation,
	mcp.ToolName("slack_rename_conversation"):      renameConversation,
	mcp.ToolName("slack_send_message"):             sendMessage,
	mcp.ToolName("slack_update_message"):           updateMessage,
	mcp.ToolName("slack_delete_message"):           deleteMessage,
	mcp.ToolName("slack_search_messages"):          searchMessages,
	mcp.ToolName("slack_add_reaction"):             addReaction,
	mcp.ToolName("slack_remove_reaction"):          removeReaction,
	mcp.ToolName("slack_get_reactions"):            getReactions,
	mcp.ToolName("slack_add_pin"):                  addPin,
	mcp.ToolName("slack_remove_pin"):               removePin,
	mcp.ToolName("slack_list_pins"):                listPins,
	mcp.ToolName("slack_schedule_message"):         scheduleMessage,
	mcp.ToolName("slack_list_users"):               listUsers,
	mcp.ToolName("slack_get_user_info"):            getUserInfo,
	mcp.ToolName("slack_get_user_presence"):        getUserPresence,
	mcp.ToolName("slack_list_user_groups"):         listUserGroups,
	mcp.ToolName("slack_get_user_group"):           getUserGroup,
	mcp.ToolName("slack_auth_test"):                authTest,
	mcp.ToolName("slack_team_info"):                teamInfo,
	mcp.ToolName("slack_upload_file"):              uploadFile,
	mcp.ToolName("slack_list_files"):               listFiles,
	mcp.ToolName("slack_delete_file"):              deleteFile,
	mcp.ToolName("slack_list_emoji"):               listEmoji,
	mcp.ToolName("slack_set_status"):               setStatus,
	mcp.ToolName("slack_list_bookmarks"):           listBookmarks,
	mcp.ToolName("slack_add_bookmark"):             addBookmark,
	mcp.ToolName("slack_remove_bookmark"):          removeBookmark,
	mcp.ToolName("slack_add_reminder"):             addReminder,
	mcp.ToolName("slack_list_reminders"):           listReminders,
	mcp.ToolName("slack_delete_reminder"):          deleteReminder,
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
