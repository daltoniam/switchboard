package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "slack", i.Name())
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveSlackPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "slack_", "tool %s missing slack_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &slackIntegration{clients: make(map[string]*slack.Client)}
	result, err := s.Execute(t.Context(), "slack_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

// --- cookie transport tests ---

func TestCookieTransport_InjectsCookie(t *testing.T) {
	var capturedCookie string
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedCookie = req.Header.Get("Cookie")
		return &http.Response{StatusCode: 200}, nil
	})

	transport := &cookieTransport{cookie: "test-cookie-value", inner: inner}
	req, _ := http.NewRequest("GET", "https://slack.com/api/test", nil)
	_, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "d=test-cookie-value", capturedCookie)
}

func TestCookieTransport_AppendsToExisting(t *testing.T) {
	var capturedCookie string
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedCookie = req.Header.Get("Cookie")
		return &http.Response{StatusCode: 200}, nil
	})

	transport := &cookieTransport{cookie: "test-cookie", inner: inner}
	req, _ := http.NewRequest("GET", "https://slack.com/api/test", nil)
	req.Header.Set("Cookie", "existing=value")
	_, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "existing=value; d=test-cookie", capturedCookie)
}

func TestCookieTransport_NoCookie(t *testing.T) {
	var capturedCookie string
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedCookie = req.Header.Get("Cookie")
		return &http.Response{StatusCode: 200}, nil
	})

	transport := &cookieTransport{cookie: "", inner: inner}
	req, _ := http.NewRequest("GET", "https://slack.com/api/test", nil)
	_, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Empty(t, capturedCookie)
}

// roundTripFunc adapts a function to the http.RoundTripper interface.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// --- helper function tests ---

func TestArgStr(t *testing.T) {
	v, err := mcp.ArgStr(map[string]any{"k": "val"}, "k")
	assert.NoError(t, err)
	assert.Equal(t, "val", v)
	v, err = mcp.ArgStr(map[string]any{}, "k")
	assert.NoError(t, err)
	assert.Empty(t, v)
}

func TestArgInt(t *testing.T) {
	v, err := mcp.ArgInt(map[string]any{"n": float64(42)}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{"n": 42}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{"n": "42"}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 0, v)
}

func TestArgBool(t *testing.T) {
	v, err := mcp.ArgBool(map[string]any{"b": true}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": false}, "b")
	assert.NoError(t, err)
	assert.False(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": "true"}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": "1"}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{}, "b")
	assert.NoError(t, err)
	assert.False(t, v)
}

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestWrapRetryable_SlackRateLimitedError(t *testing.T) {
	rle := &slack.RateLimitedError{RetryAfter: 30 * time.Second}
	wrapped := wrapRetryable(rle)
	require.Error(t, wrapped)

	var re *mcp.RetryableError
	require.ErrorAs(t, wrapped, &re)
	assert.Equal(t, 429, re.StatusCode)
	assert.Equal(t, 30*time.Second, re.RetryAfter)
}

func TestWrapRetryable_NonRetryableError(t *testing.T) {
	err := fmt.Errorf("channel not found")
	wrapped := wrapRetryable(err)
	assert.Equal(t, err, wrapped, "non-retryable errors should pass through unchanged")

	assert.False(t, mcp.IsRetryable(wrapped))
}

func TestWrapRetryable_NilError(t *testing.T) {
	assert.Nil(t, wrapRetryable(nil))
}

func TestErrResult_PropagatesSlackRateLimitedError(t *testing.T) {
	rle := &slack.RateLimitedError{RetryAfter: 10 * time.Second}
	result, err := errResult(rle)
	assert.Nil(t, result, "retryable error should not produce a ToolResult")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

// --- multi-workspace token store tests ---

func TestTokenStore_SetAndGetWorkspace(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T1", TeamName: "Acme", Token: "xoxc-1", Cookie: "xoxd-1"})

	ws := ts.getWorkspace("T1")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxc-1", ws.Token)
	assert.Equal(t, "Acme", ws.TeamName)
}

func TestTokenStore_DefaultWorkspace(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-1"})
	ts.setWorkspace(&workspace{TeamID: "T2", Token: "xoxc-2"})

	assert.Equal(t, "T1", ts.defaultID(), "first workspace set becomes default")

	ws := ts.getDefault()
	require.NotNil(t, ws)
	assert.Equal(t, "T1", ws.TeamID)

	ts.setDefault("T2")
	ws = ts.getDefault()
	require.NotNil(t, ws)
	assert.Equal(t, "T2", ws.TeamID)
}

func TestTokenStore_BackwardCompatGet(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T1", Token: "xoxc-tok", Cookie: "xoxd-cook"})

	tok, cook := ts.get()
	assert.Equal(t, "xoxc-tok", tok)
	assert.Equal(t, "xoxd-cook", cook)
}

func TestTokenStore_SaveAndLoadV2(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tokens.json")

	ts := &tokenStore{workspaces: make(map[string]*workspace), filePath: fp}
	ts.setWorkspace(&workspace{TeamID: "T1", TeamName: "Acme", Token: "xoxc-1", Cookie: "xoxd-1", Source: "chrome"})
	ts.setWorkspace(&workspace{TeamID: "T2", TeamName: "Beta", Token: "xoxc-2", Cookie: "xoxd-2", Source: "web"})
	ts.setDefault("T2")

	require.NoError(t, ts.saveToFile())

	ts2 := &tokenStore{workspaces: make(map[string]*workspace), filePath: fp}
	ts2.loadFromFile()

	assert.Equal(t, "T2", ts2.defaultID())
	assert.Len(t, ts2.allWorkspaces(), 2)

	ws1 := ts2.getWorkspace("T1")
	require.NotNil(t, ws1)
	assert.Equal(t, "Acme", ws1.TeamName)
	assert.Equal(t, "xoxc-1", ws1.Token)
}

func TestTokenStore_LoadLegacyFormat(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tokens.json")

	legacy := map[string]string{
		"token":      "xoxc-legacy",
		"cookie":     "xoxd-legacy",
		"team_id":    "TLEGACY",
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	require.NoError(t, os.WriteFile(fp, data, 0600))

	ts := &tokenStore{workspaces: make(map[string]*workspace), filePath: fp}
	ts.loadFromFile()

	assert.Equal(t, "TLEGACY", ts.defaultID())
	ws := ts.getWorkspace("TLEGACY")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxc-legacy", ws.Token)
	assert.Equal(t, "xoxd-legacy", ws.Cookie)
}

func TestTokenStore_AllWorkspaces(t *testing.T) {
	ts := &tokenStore{workspaces: make(map[string]*workspace)}
	ts.setWorkspace(&workspace{TeamID: "T2", TeamName: "Beta", Token: "xoxc-2"})
	ts.setWorkspace(&workspace{TeamID: "T1", TeamName: "Alpha", Token: "xoxc-1"})

	all := ts.allWorkspaces()
	require.Len(t, all, 2)
	assert.Equal(t, "Alpha", all[0].TeamName, "should be sorted by name")
	assert.Equal(t, "Beta", all[1].TeamName)
}

func TestGetClientForArgs_DefaultWorkspace(t *testing.T) {
	s := &slackIntegration{
		clients: map[string]*slack.Client{"T1": slack.New("xoxc-1")},
		store:   &tokenStore{workspaces: make(map[string]*workspace), defaultTeamID: "T1"},
	}

	client, err := s.getClientForArgs(map[string]any{})
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGetClientForArgs_SpecificWorkspace(t *testing.T) {
	s := &slackIntegration{
		clients: map[string]*slack.Client{
			"T1": slack.New("xoxc-1"),
			"T2": slack.New("xoxc-2"),
		},
		store: &tokenStore{workspaces: make(map[string]*workspace), defaultTeamID: "T1"},
	}

	client, err := s.getClientForArgs(map[string]any{"team_id": "T2"})
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestGetClientForArgs_UnknownWorkspace(t *testing.T) {
	s := &slackIntegration{
		clients: map[string]*slack.Client{"T1": slack.New("xoxc-1")},
		store:   &tokenStore{workspaces: make(map[string]*workspace), defaultTeamID: "T1"},
	}

	_, err := s.getClientForArgs(map[string]any{"team_id": "T_UNKNOWN"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown workspace")
}

// --- Configure token-gating tests ---

func TestConfigure_ConfigTokenSkipsFileAndChrome(t *testing.T) {
	s := &slackIntegration{}
	s.Configure(t.Context(), mcp.Credentials{
		"token":   "xoxb-test-token",
		"team_id": "T_CONFIG",
	})

	// Exactly one workspace with Source=="config".
	all := s.store.allWorkspaces()
	require.Len(t, all, 1)
	assert.Equal(t, "config", all[0].Source)
	assert.Equal(t, "xoxb-test-token", all[0].Token)
	assert.Equal(t, "T_CONFIG", all[0].TeamID)

	// No background refresh started.
	assert.Nil(t, s.stopBg)
}

func TestConfigure_ConfigTokenDefaultTeamID(t *testing.T) {
	s := &slackIntegration{}
	s.Configure(t.Context(), mcp.Credentials{
		"token": "xoxb-test-token",
	})

	// When no team_id is provided, it defaults to "_config".
	ws := s.store.getWorkspace("_config")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxb-test-token", ws.Token)
}

func TestConfigure_SwitchToConfigTokenStopsBackgroundRefresh(t *testing.T) {
	stopCh := make(chan struct{})
	s := &slackIntegration{
		stopBg: stopCh,
	}
	s.Configure(t.Context(), mcp.Credentials{
		"token":   "xoxb-test-token",
		"team_id": "T_CONFIG",
	})

	// The pre-existing stopBg channel should have been closed.
	select {
	case <-stopCh:
		// ok — channel was closed
	default:
		t.Fatal("expected previous stopBg channel to be closed")
	}

	// And stopBg is now nil (no new refresh goroutine).
	assert.Nil(t, s.stopBg)
}

// --- Browser-snapshot config tokens (xoxc-* with token_source=browser) ---
// These rotate constantly and must participate in the local refresh loop,
// even when bootstrapped via config.

func TestConfigure_BrowserConfigTokenMarksSourceBrowser(t *testing.T) {
	// Point the file store somewhere empty so loadFromFile is a no-op.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	s := &slackIntegration{}
	s.Configure(t.Context(), mcp.Credentials{
		"token":                "xoxc-rotating-session-token",
		"team_id":              "T_BROWSER",
		mcp.CredKeyTokenSource: "browser",
	})

	ws := s.store.getWorkspace("T_BROWSER")
	require.NotNil(t, ws)
	assert.Equal(t, "browser", ws.Source, "browser-source xoxc-* config token should not be marked source=config")
	assert.NotNil(t, s.stopBg, "background refresh should run for browser-snapshot tokens")

	// And canSelfRefresh should allow it.
	assert.True(t, s.canSelfRefresh(ws))
}

func TestConfigure_XoxcConfigTokenWithoutSourceFlagIsStillBrowser(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	s := &slackIntegration{}
	// xoxc- prefix alone is structurally a browser session token; treat it
	// as such even if the token_source flag was lost.
	s.Configure(t.Context(), mcp.Credentials{
		"token":   "xoxc-rotating-session-token",
		"team_id": "T_XOXC",
	})

	ws := s.store.getWorkspace("T_XOXC")
	require.NotNil(t, ws)
	assert.Equal(t, "browser", ws.Source)
	assert.NotNil(t, s.stopBg)
}

func TestConfigure_FileTokenWinsOverConfigForSameTeamID(t *testing.T) {
	// Persisted file carries the fresher xoxc-* token; config token is the
	// stale bootstrap copy. After Configure(), the file's token must win.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	fp := filepath.Join(tmp, ".slack-mcp-tokens.json")
	fileJSON := `{
	  "version": 2,
	  "default_team_id": "T_DUP",
	  "workspaces": [
	    {
	      "team_id": "T_DUP",
	      "team_name": "Fresh Workspace",
	      "token": "xoxc-FRESH-from-file",
	      "cookie": "xoxd-fresh",
	      "source": "chrome",
	      "updated_at": "` + time.Now().UTC().Format(time.RFC3339) + `"
	    }
	  ]
	}`
	require.NoError(t, os.WriteFile(fp, []byte(fileJSON), 0600))

	s := &slackIntegration{}
	s.Configure(t.Context(), mcp.Credentials{
		"token":                "xoxc-STALE-from-config",
		"team_id":              "T_DUP",
		mcp.CredKeyTokenSource: "browser",
	})

	ws := s.store.getWorkspace("T_DUP")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxc-FRESH-from-file", ws.Token, "fresher file token should overwrite stale config token")
}

func TestConfigure_OAuthConfigTokenStillSkipsFile(t *testing.T) {
	// xoxb-/xoxp- tokens are externally managed and must NOT be replaced
	// by file-loaded entries — keep the "config is authoritative" path.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	fp := filepath.Join(tmp, ".slack-mcp-tokens.json")
	fileJSON := `{"version":2,"workspaces":[{"team_id":"T_OAUTH","token":"xoxc-from-file","source":"chrome","updated_at":"` + time.Now().UTC().Format(time.RFC3339) + `"}]}`
	require.NoError(t, os.WriteFile(fp, []byte(fileJSON), 0600))

	s := &slackIntegration{}
	s.Configure(t.Context(), mcp.Credentials{
		"token":   "xoxb-bot-from-config",
		"team_id": "T_OAUTH",
	})

	ws := s.store.getWorkspace("T_OAUTH")
	require.NotNil(t, ws)
	assert.Equal(t, "xoxb-bot-from-config", ws.Token)
	assert.Equal(t, "config", ws.Source)
	assert.Nil(t, s.stopBg, "no background refresh for OAuth/bot config tokens")
}

func TestErrResult_NonRetryableProducesToolResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("no workspace"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "no workspace", result.Data)
}

// --- canSelfRefresh policy ---

func TestTokenPrefix(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "<empty>"},
		{"short xoxc-", "xoxc-", "xoxc-"},
		{"xoxc with body", "xoxc-225638936291-secret-stuff", "xoxc-…"},
		{"xoxp with body", "xoxp-abc-def", "xoxp-…"},
		{"xoxd cookie", "xoxd-encryptedblob", "xoxd-…"},
		{"only prefix length", "xoxb-", "xoxb-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenPrefix(tc.in)
			assert.Equal(t, tc.want, got)
			// Stronger guarantee: the secret body never leaks.
			if len(tc.in) > 5 {
				assert.NotContains(t, got, tc.in[5:], "tokenPrefix leaked secret body")
			}
		})
	}
}

func TestCanSelfRefresh(t *testing.T) {
	s := &slackIntegration{}
	cases := []struct {
		name string
		ws   *workspace
		want bool
	}{
		{"nil workspace", nil, false},
		{"config source", &workspace{Token: "xoxb-1", Source: "config"}, false},
		{"OAuth user token", &workspace{Token: "xoxp-1", Source: "chrome"}, false},
		{"browser session token", &workspace{Token: "xoxc-1", Source: "chrome"}, true},
		{"slack desktop source", &workspace{Token: "xoxc-1", Source: "slack"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, s.canSelfRefresh(tc.ws))
		})
	}
}

// --- resolveWorkspaceIdentities self-healing ---

// newAuthTestServer returns an httptest server that fails the first N
// auth.test calls with invalid_auth, then succeeds with the given team_id.
func newAuthTestServer(t *testing.T, teamID string, failFirstN int32) *httptest.Server {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) <= failFirstN {
			_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
			return
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"team":"Test","team_id":%q,"user":"u","user_id":"U1","url":"https://test.slack.com/"}`, teamID)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newClientForAPI(apiURL string) *slack.Client {
	// slack-go's OptionAPIURL expects a trailing slash.
	u := apiURL
	if u[len(u)-1] != '/' {
		u += "/"
	}
	if _, err := url.Parse(u); err != nil {
		panic(err)
	}
	return slack.New("xoxc-stale", slack.OptionAPIURL(u))
}

func TestResolveWorkspaceIdentities_RecoversAfterRefresh(t *testing.T) {
	teamID := "T6MJSTJ8K"
	srv := newAuthTestServer(t, teamID, 1) // first call fails, second succeeds

	s := &slackIntegration{
		clients: map[string]*slack.Client{teamID: newClientForAPI(srv.URL)},
		store:   &tokenStore{workspaces: map[string]*workspace{teamID: {TeamID: teamID, Token: "xoxc-stale", Source: "chrome"}}},
	}

	var refreshCalls atomic.Int32
	s.refreshWorkspace = func(id string) bool {
		assert.Equal(t, teamID, id)
		refreshCalls.Add(1)
		// Simulate a successful refresh by replacing the client with a fresh
		// one that points at the same test server (which will now succeed).
		s.mu.Lock()
		s.clients[id] = newClientForAPI(srv.URL)
		s.mu.Unlock()
		return true
	}

	s.resolveWorkspaceIdentities(context.Background())

	assert.Equal(t, int32(1), refreshCalls.Load(), "refresh should be attempted exactly once")
	ws := s.store.getWorkspace(teamID)
	require.NotNil(t, ws)
	assert.Equal(t, "Test", ws.TeamName, "team name should be filled in from successful auth.test")
}

func TestResolveWorkspaceIdentities_GivesUpWhenRefreshFails(t *testing.T) {
	teamID := "T1"
	srv := newAuthTestServer(t, teamID, 99) // always fails

	s := &slackIntegration{
		clients: map[string]*slack.Client{teamID: newClientForAPI(srv.URL)},
		store:   &tokenStore{workspaces: map[string]*workspace{teamID: {TeamID: teamID, Token: "xoxc-stale", Source: "chrome"}}},
	}
	s.refreshWorkspace = func(string) bool { return false }

	// Should not panic; workspace should remain present with no team name.
	s.resolveWorkspaceIdentities(context.Background())
	ws := s.store.getWorkspace(teamID)
	require.NotNil(t, ws)
	assert.Empty(t, ws.TeamName)
}

func TestResolveWorkspaceIdentities_SkipsRefreshForConfigToken(t *testing.T) {
	teamID := "T_CFG"
	srv := newAuthTestServer(t, teamID, 99) // always fails

	s := &slackIntegration{
		clients: map[string]*slack.Client{teamID: newClientForAPI(srv.URL)},
		store:   &tokenStore{workspaces: map[string]*workspace{teamID: {TeamID: teamID, Token: "xoxb-cfg", Source: "config"}}},
	}
	var called atomic.Bool
	s.refreshWorkspace = func(string) bool { called.Store(true); return true }

	s.resolveWorkspaceIdentities(context.Background())
	assert.False(t, called.Load(), "config-sourced tokens must not trigger local refresh")
}

func TestResolveWorkspaceIdentities_SkipsRefreshForOAuthToken(t *testing.T) {
	teamID := "T_OAUTH"
	srv := newAuthTestServer(t, teamID, 99)

	s := &slackIntegration{
		clients: map[string]*slack.Client{teamID: newClientForAPI(srv.URL)},
		store:   &tokenStore{workspaces: map[string]*workspace{teamID: {TeamID: teamID, Token: "xoxp-user", Source: "chrome"}}},
	}
	var called atomic.Bool
	s.refreshWorkspace = func(string) bool { called.Store(true); return true }

	s.resolveWorkspaceIdentities(context.Background())
	assert.False(t, called.Load(), "OAuth user tokens must not trigger local refresh")
}
