package slack

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/slack-go/slack"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("slack", compactYAML, compact.Options{Strict: false})
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
	bgDone  sync.WaitGroup
	revoked *revokedCredentials

	// lifecycleMu serializes Configure and Stop, which the setup page calls
	// while tool calls run.
	lifecycleMu sync.Mutex

	// refreshWorkspace is overridable in tests; defaults to (*slackIntegration).tryRefreshWorkspace.
	refreshWorkspace func(ctx context.Context, teamID string) bool
	// cookieRefresh is overridable in tests; defaults to (*slackIntegration).tryRefreshViaCookieForTeam.
	cookieRefresh func(ctx context.Context, teamID string) bool
}

func New() mcp.Integration {
	return &slackIntegration{}
}

func (s *slackIntegration) Name() string { return "slack" }

func (s *slackIntegration) Configure(ctx context.Context, creds mcp.Credentials) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	store := newTokenStore()
	revoked, err := newRevokedCredentials(filepath.Join(filepath.Dir(store.filePath), filepath.Base(revokedFilePath())))
	if err != nil {
		return err
	}

	configToken := creds["token"]
	// xoxc-* tokens are browser session tokens that rotate constantly. Even
	// when bootstrapped from config, they must participate in the local
	// refresh/file-persistence loop or they go stale within hours.
	//   - token_source: "browser"            → explicitly a browser snapshot
	//   - token has xoxc- prefix             → structurally a session token
	// Genuine externally-managed tokens (xoxb-, xoxp-, OAuth) keep the old
	// "config is authoritative" behavior so we never clobber them.
	isBrowserConfig := configToken != "" &&
		(creds[mcp.CredKeyTokenSource] == "browser" || parseCredKind(configToken) == kindBrowserSession)

	if configToken != "" {
		teamID := creds["team_id"]
		if teamID == "" {
			teamID = "_config"
		}
		source := "config"
		if isBrowserConfig {
			source = "browser"
		}
		store.setWorkspace(&workspace{
			TeamID: teamID,
			Token:  configToken,
			Cookie: creds["cookie"],
			Source: source,
		})
		if creds["team_id"] != "" {
			store.setDefault(creds["team_id"])
		}
	}

	// When the config token is a rotating browser snapshot, also load the
	// persisted file. The file may carry a fresher copy (background refresh
	// writes there) and we want that to win.
	if configToken == "" || isBrowserConfig {
		store.loadFromFile()

		if len(store.allWorkspaces()) == 0 {
			wss, _ := listWorkspacesFromAllBrowsers()
			for _, ws := range wss {
				store.setWorkspace(&workspace{
					TeamID:   ws.TeamID,
					TeamName: ws.Name,
					Source:   "chrome",
				})
			}
		}
	}

	if len(store.allWorkspaces()) == 0 {
		return fmt.Errorf("slack: no token found — run with --web to configure, set SLACK_TOKEN/SLACK_COOKIE env vars, or open Slack in Chrome (macOS)")
	}

	// Stop the old refresh worker before installing the new state, so a
	// refresh in flight cannot write old credentials over the new ones.
	s.stopBackgroundRefresh()

	// Tool calls read s.store and s.revoked without lifecycleMu, so a reload
	// fills the existing store in place. The revoked record is one shared
	// instance per path, so after the first Configure the pointer never changes.
	if s.store == nil {
		s.store = store
	} else {
		s.store.replaceWith(store)
	}
	if s.revoked != revoked {
		s.revoked = revoked
	}
	s.mu.Lock()
	s.clients = make(map[string]*slack.Client)
	s.mu.Unlock()

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
		s.startBackgroundRefresh()
	}

	return nil
}

// Stop ends the background cookie refresh and waits for an in-flight
// refresh to finish. The setup page calls it when the user disables Slack.
func (s *slackIntegration) Stop() {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.stopBackgroundRefresh()
}

func (s *slackIntegration) startBackgroundRefresh() {
	stop := make(chan struct{})
	s.stopBg = stop
	s.bgDone.Add(1)
	go func() {
		defer s.bgDone.Done()
		s.backgroundRefresh(stop)
	}()
}

func (s *slackIntegration) stopBackgroundRefresh() {
	if s.stopBg == nil {
		return
	}
	close(s.stopBg)
	s.stopBg = nil
	s.bgDone.Wait()
}

func (s *slackIntegration) buildAllClients() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ws := range s.store.allWorkspaces() {
		transport := newCookieTransport(ws, s.revoked)
		s.clients[ws.TeamID] = slack.New(ws.Token, slack.OptionHTTPClient(&http.Client{Transport: transport}))
	}
}

func (s *slackIntegration) buildClientForWorkspace(ws *workspace) {
	transport := newCookieTransport(ws, s.revoked)
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
	if !s.refreshFn()(ctx, ws.TeamID) {
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
	return parseCredKind(ws.Token) == kindBrowserSession
}

// refreshFn returns the workspace-refresh callback, defaulting to the real
// browser/cookie path. Tests override s.refreshWorkspace for determinism.
func (s *slackIntegration) refreshFn() func(ctx context.Context, teamID string) bool {
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

// backgroundRefreshInterval is replaced in tests.
var backgroundRefreshInterval = 4 * time.Hour

func (s *slackIntegration) backgroundRefresh(stop <-chan struct{}) {
	// Closing stop also cancels a refresh in flight, so Stop never waits on
	// a stalled request to Slack.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	ticker := time.NewTicker(backgroundRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.tryRefresh(ctx)
		case <-stop:
			return
		}
	}
}

func (s *slackIntegration) tryRefresh(ctx context.Context) bool {
	allOk := true
	refresh := s.refreshFn()
	for _, ws := range s.store.allWorkspaces() {
		if !s.canSelfRefresh(ws) {
			continue
		}
		if !refresh(ctx, ws.TeamID) {
			allOk = false
		}
	}
	return allOk
}

// extractFromBrowserFn is replaced in tests so no test reads real browser data.
var extractFromBrowserFn = extractFromBrowser

// credentialKey is the value the revoked record tracks for a workspace: the
// token and cookie together for a browser session, so one workspace's stale
// token does not block the desktop cookie other workspaces share; otherwise
// the token alone (user and bot tokens carry no cookie).
func credentialKey(ws *workspace) string {
	if parseCredKind(ws.Token) == kindBrowserSession && ws.Cookie != "" {
		return ws.Token + "\x00" + ws.Cookie
	}
	return ws.Token
}

func (s *slackIntegration) tryRefreshWorkspace(ctx context.Context, teamID string) bool {
	// A rejected workspace stays put until the user replaces its credential.
	// Pulling a fresh browser cookie here and sending it logs strict
	// workspaces out again.
	if ws := s.store.getWorkspace(teamID); ws != nil && s.revoked.isRevoked(credentialKey(ws)) {
		log.Printf("slack: workspace %s credential was rejected — not refreshing; replace it on the Slack setup page", teamID)
		return false
	}
	cookieRefresh := s.tryRefreshViaCookieForTeam
	if s.cookieRefresh != nil {
		cookieRefresh = s.cookieRefresh
	}
	if cookieRefresh(ctx, teamID) {
		return true
	}
	// The cookie refresh can store a new pair that Slack then rejects.
	// Extracting yet another browser pair would bypass that rejection.
	if ws := s.store.getWorkspace(teamID); ws != nil && s.revoked.isRevoked(credentialKey(ws)) {
		log.Printf("slack: workspace %s refreshed credential was rejected — not extracting from the browser; replace it on the Slack setup page", teamID)
		return false
	}
	extracted := extractFromBrowserFn(teamID)
	if extracted == nil || extracted.token == "" {
		return false
	}
	cookie := extracted.cookie
	var oldToken, oldCookie string
	if ws := s.store.getWorkspace(teamID); ws != nil {
		oldToken, oldCookie = ws.Token, ws.Cookie
		if cookie == "" {
			cookie = ws.Cookie
		}
	}
	if !s.store.replaceTokensIf(teamID, oldToken, oldCookie, extracted.token, cookie) {
		log.Printf("slack: workspace %s credentials changed during browser extraction — discarding the extracted pair", teamID)
		return false
	}
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
	revoked *revokedCredentials
	key     string // credentialKey of the workspace this transport serves
	cookie  string
	inner   http.RoundTripper
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.revoked.isRevoked(t.key) {
		return nil, errRevokedCredential
	}
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
	resp, err := t.inner.RoundTrip(req)
	if err == nil {
		t.revoked.markIfInvalidAuth(t.key, resp)
	}
	return resp, err
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
	mcp.ToolName("slack_lookup_user_by_email"):     lookupUserByEmail,
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
