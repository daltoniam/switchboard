package slack

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compactyaml"
	"github.com/slack-go/slack"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compactyaml.MustLoadWithOverlay("slack", compactYAML, compactyaml.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// Compile-time interface assertions.
var (
	_ mcp.Integration                = (*slackIntegration)(nil)
	_ mcp.FieldCompactionIntegration = (*slackIntegration)(nil)
	_ mcp.PlainTextCredentials       = (*slackIntegration)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*slackIntegration)(nil)
)

func (s *slackIntegration) PlainTextKeys() []string {
	return []string{"team_id"}
}

type slackIntegration struct {
	mu      sync.RWMutex
	clients map[string]*slack.Client // keyed by team_id
	store   *tokenStore
	stopBg  chan struct{}

	// refreshWorkspace is overridable in tests; defaults to (*slackIntegration).tryRefreshWorkspace.
	refreshWorkspace func(teamID string) bool
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
	// xoxc-* tokens are browser session tokens that rotate constantly. Even
	// when bootstrapped from config, they must participate in the local
	// refresh/file-persistence loop or they go stale within hours.
	//   - token_source: "browser"            → explicitly a browser snapshot
	//   - token has xoxc- prefix             → structurally a session token
	// Genuine externally-managed tokens (xoxb-, xoxp-, OAuth) keep the old
	// "config is authoritative" behavior so we never clobber them.
	isBrowserConfig := configToken != "" &&
		(creds[mcp.CredKeyTokenSource] == "browser" || strings.HasPrefix(configToken, "xoxc-"))

	if configToken != "" {
		teamID := creds["team_id"]
		if teamID == "" {
			teamID = "_config"
		}
		source := "config"
		if isBrowserConfig {
			source = "browser"
		}
		s.store.setWorkspace(&workspace{
			TeamID: teamID,
			Token:  configToken,
			Cookie: creds["cookie"],
			Source: source,
		})
		if creds["team_id"] != "" {
			s.store.setDefault(creds["team_id"])
		}
	}

	// When the config token is a rotating browser snapshot, also load the
	// persisted file. The file may carry a fresher copy (background refresh
	// writes there) and we want that to win.
	if configToken == "" || isBrowserConfig {
		s.store.loadFromFile()

		if len(s.store.allWorkspaces()) == 0 {
			wss, _ := listWorkspacesFromAllBrowsers()
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

	// Persist + run background refresh for any non-OAuth-style token (file-
	// loaded or browser-sourced from config). Only genuine externally-managed
	// tokens skip this.
	if configToken == "" || isBrowserConfig {
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
			recovered, retryResp := s.tryRecoverAuth(ctx, ws, err)
			if !recovered {
				continue
			}
			resp = retryResp
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

// tryRecoverAuth attempts a one-shot self-refresh for a workspace whose
// startup auth.test just failed. xoxc-* browser session tokens rotate
// frequently and the fresh token usually sits in the browser's local storage;
// config-provided and OAuth (xoxp-*) tokens are managed externally and must
// never be clobbered by a local refresh. Returns (true, newResp) when the
// post-refresh auth.test succeeds, (false, nil) otherwise (caller logs the
// original error and gives up).
func (s *slackIntegration) tryRecoverAuth(ctx context.Context, ws *workspace, origErr error) (bool, *slack.AuthTestResponse) {
	if !s.canSelfRefresh(ws) {
		log.Printf("slack: auth test failed for workspace %s: %v (skipping self-refresh: source=%s token=%s)", ws.TeamID, origErr, ws.Source, tokenPrefix(ws.Token))
		return false, nil
	}
	log.Printf("slack: auth test failed for workspace %s: %v — attempting startup refresh", ws.TeamID, origErr)
	if !s.refreshFn()(ws.TeamID) {
		log.Printf("slack: startup refresh failed for workspace %s — manual re-extract may be required (open Slack in Chrome and re-extract via web UI)", ws.TeamID)
		return false, nil
	}
	// Defensive: tryRefreshWorkspace's success path always calls
	// buildClientForWorkspace (see tryRefreshWorkspace + tryRefreshViaCookieForTeam),
	// so this branch is unreachable under the current implementation. Kept to
	// avoid a startup-goroutine panic if that invariant ever regresses.
	c := s.getClientForTeam(ws.TeamID)
	if c == nil {
		log.Printf("slack: startup refresh produced no client for workspace %s", ws.TeamID)
		return false, nil
	}
	resp, err := c.AuthTestContext(ctx)
	if err != nil {
		log.Printf("slack: post-refresh auth test still failing for workspace %s: %v", ws.TeamID, err)
		return false, nil
	}
	log.Printf("slack: auth recovered for workspace %s after startup refresh", ws.TeamID)
	return true, resp
}

// tokenPrefix returns the leading non-secret portion of a Slack token for
// log diagnostics. Slack token type prefixes ("xoxc-", "xoxp-", "xoxb-",
// "xoxd-") are 5 bytes; anything past that is the secret body and is
// replaced with an ellipsis.
func tokenPrefix(tok string) string {
	const prefixLen = 5 // len("xoxc-")
	if tok == "" {
		return "<empty>"
	}
	if len(tok) <= prefixLen {
		return tok
	}
	return tok[:prefixLen] + "…"
}

func (s *slackIntegration) getClientForTeam(teamID string) *slack.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if teamID == "" {
		teamID = s.store.defaultID()
	}
	return s.clients[teamID]
}

// canSelfRefresh reports whether the workspace's token may be safely replaced
// by a locally-sourced (cookie/browser) refresh. Config-provided tokens are
// managed externally; OAuth (xoxp-*) tokens do not rotate and a refresh path
// for them does not exist.
func (s *slackIntegration) canSelfRefresh(ws *workspace) bool {
	if ws == nil || ws.Source == "config" {
		return false
	}
	// Only xoxc-* browser session tokens rotate and have a local refresh
	// path. xoxb-/xoxp-/xapp-/etc. are externally managed and must never
	// be replaced by a browser extract, even when stored locally.
	return strings.HasPrefix(ws.Token, "xoxc-")
}

// refreshFn returns the workspace-refresh callback, defaulting to the real
// browser/cookie path. Tests override s.refreshWorkspace for determinism.
func (s *slackIntegration) refreshFn() func(teamID string) bool {
	if s.refreshWorkspace != nil {
		return s.refreshWorkspace
	}
	return s.tryRefreshWorkspace
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

func (s *slackIntegration) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
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
	refresh := s.refreshFn()
	for _, ws := range s.store.allWorkspaces() {
		if !s.canSelfRefresh(ws) {
			continue
		}
		if !refresh(ws.TeamID) {
			allOk = false
		}
	}
	return allOk
}

func (s *slackIntegration) tryRefreshWorkspace(teamID string) bool {
	if s.tryRefreshViaCookieForTeam(teamID) {
		return true
	}
	extracted := extractFromBrowser(teamID)
	if extracted == nil || extracted.token == "" {
		return false
	}
	cookie := extracted.cookie
	if cookie == "" {
		if ws := s.store.getWorkspace(teamID); ws != nil {
			cookie = ws.Cookie
		}
	}
	s.store.updateTokens(teamID, extracted.token, cookie)
	ws := s.store.getWorkspace(teamID)
	if ws != nil {
		s.buildClientForWorkspace(ws)
	}
	_ = s.store.saveToFile()
	log.Printf("slack: tokens refreshed from %s for %s", extracted.source, teamID)
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
