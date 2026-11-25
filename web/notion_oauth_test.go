package web

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	notionInt "github.com/daltoniam/switchboard/integrations/notion"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotionOAuthProfile(t *testing.T) {
	profile, ok := remoteOAuthProfiles["notion"]
	require.True(t, ok)
	assert.Equal(t, "default", profile.Options.Scope)
	assert.Equal(t, "/mcp", profile.ResourcePath)
	assert.Equal(t, "/api/remote/notion/oauth/callback", profile.CallbackPath)
	assert.Equal(t, "mcp_access_token", profile.CredentialKey)
	assert.Empty(t, profile.ClearCredentialKey)
}

func TestNotionSetupSignIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		ic   *mcp.IntegrationConfig
		want []string
	}{
		{name: "missing config", want: []string{"Not Connected", "Sign in with Notion →"}},
		{name: "browser token", ic: &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"token_v2": "private-cookie"}}, want: []string{"Invalid Token", "Sign in with Notion →"}},
		{name: "signed in", ic: &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"mcp_access_token": "private-token"}}, want: []string{"Invalid Token", "Re-authorize with Notion"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			reg.integrations["notion"] = &mockIntegration{name: "notion"}
			if tc.ic != nil {
				cfg.cfg.Integrations["notion"] = tc.ic
			}
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/notion/setup", nil))
			require.Equal(t, http.StatusOK, rr.Code)
			body := rr.Body.String()
			for _, text := range append([]string{"Notion Setup", "Option 1: Sign in with Notion (recommended)", "/api/remote/notion/oauth/start", "/api/notion/save-token", `aria-live="polite"`, "Try Again"}, tc.want...) {
				assert.Contains(t, body, text)
			}
			assert.NotContains(t, body, "notion-mcp")
			assert.NotContains(t, body, "private-token")
			assert.NotContains(t, body, "private-cookie")
		})
	}
}

func TestNotionSaveTokenReplacesSignIn(t *testing.T) {
	ws, reg, cfg := setupTestWeb()
	reg.integrations["notion"] = &mockIntegration{name: "notion"}
	cfg.cfg.Integrations["notion"] = &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"mcp_access_token": "hosted-token", "mcp_base_url": "https://proxy.test", mcp.CredKeyTokenSource: "oauth"}}
	rr := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/notion/save-token", strings.NewReader(url.Values{"token_v2": {"v03:cookie"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ws.Handler().ServeHTTP(rr, request)
	require.Equal(t, http.StatusSeeOther, rr.Code)
	saved, ok := cfg.GetIntegration("notion")
	require.True(t, ok)
	assert.Equal(t, "v03:cookie", saved.Credentials["token_v2"])
	assert.Empty(t, saved.Credentials["mcp_access_token"], "the browser token only takes effect once the hosted sign-in is cleared")
	assert.Empty(t, saved.Credentials[mcp.CredKeyTokenSource])
	assert.Equal(t, "https://proxy.test", saved.Credentials["mcp_base_url"])
}

func TestNotionDetailRedirect(t *testing.T) {
	for _, name := range []string{"notion-mcp", "notionmcp", "slackmcp"} {
		t.Run(name, func(t *testing.T) {
			ws, _, _ := setupTestWeb()
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/"+name, nil))
			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	}
	ws, reg, _ := setupTestWeb()
	reg.integrations["notion"] = notionInt.New()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/notion", nil))
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations/notion/setup", rr.Header().Get("Location"))
}

type notionOAuthFixture struct {
	server       *httptest.Server
	mu           sync.Mutex
	registration map[string]any
	tokenForm    url.Values
	discoveries  int
	tokenCalls   int
	tokenStatus  int
	toolCalls    int
	toolArgs     json.RawMessage
}

func newNotionOAuthFixture(t *testing.T, tokenStatus int) *notionOAuthFixture {
	t.Helper()
	f := &notionOAuthFixture{tokenStatus: tokenStatus}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "notion-fixture", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{Name: "notion-search", Description: "Search pages", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"},
	}}, func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		f.mu.Lock()
		f.toolCalls++
		f.toolArgs = append(json.RawMessage(nil), req.Params.Arguments...)
		f.mu.Unlock()
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"id":"fixture-page"}`}}}, nil
	})
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"resource": f.server.URL + "/mcp", "scopes_supported": []string{"default"}, "authorization_servers": []string{f.server.URL}})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.discoveries++
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/authorize",
			"token_endpoint": f.server.URL + "/token", "registration_endpoint": f.server.URL + "/register",
			"code_challenge_methods_supported": []string{"S256"}, "scopes_supported": []string{"default"},
		})
	})
	mux.HandleFunc("POST /register", func(w http.ResponseWriter, r *http.Request) {
		var registration map[string]any
		if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.registration = registration
		f.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]string{"client_id": "notion-client"})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.tokenCalls++
		f.tokenForm = r.PostForm
		f.mu.Unlock()
		if f.tokenStatus != http.StatusOK {
			writeJSON(w, f.tokenStatus, map[string]string{"error": "invalid_grant"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"access_token": "notion-oauth-token", "token_type": "Bearer"})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer notion-oauth-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func startNotionOAuth(t *testing.T, ws *WebServer) url.Values {
	t.Helper()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/remote/notion/oauth/start", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var response map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	require.Empty(t, response["error"])
	require.NotEmpty(t, response["authorize_url"])
	authorizeURL, err := url.Parse(response["authorize_url"])
	require.NoError(t, err)
	return authorizeURL.Query()
}

func notionOAuthCallback(t *testing.T, ws *WebServer, query url.Values) *url.URL {
	t.Helper()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/remote/notion/oauth/callback?"+query.Encode(), nil))
	require.Equal(t, http.StatusSeeOther, rr.Code)
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/integrations/notion/setup", location.Path)
	return location
}

func TestNotionOAuthFlow(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			f := newNotionOAuthFixture(t, http.StatusOK)
			ws, reg, _ := setupTestWeb()
			t.Setenv("HOME", t.TempDir())
			store, err := config.NewManager()
			require.NoError(t, err)
			ws.services.Config = store
			initial := &mcp.IntegrationConfig{Enabled: enabled, Credentials: mcp.Credentials{"token_v2": "legacy-token", "mcp_base_url": f.server.URL, "mcp_access_token": "previous-token"}, ToolGlobs: []string{"notion_*"}}
			require.NoError(t, store.SetIntegration("notion", initial))
			integration := notionHostedAt(t, f.server.URL)
			reg.integrations["notion"] = integration
			ws.health.entries["notion"] = healthEntry{Enabled: true, Healthy: true, CheckedAt: time.Now()}
			hooks := 0
			ws.onConfigChange = func() {
				hooks++
				durable, loadErr := config.NewManager()
				require.NoError(t, loadErr)
				saved, ok := durable.GetIntegration("notion")
				require.True(t, ok)
				assert.Equal(t, "notion-oauth-token", saved.Credentials["mcp_access_token"])
				assert.True(t, saved.Enabled, "OAuth must durably enable the adapter before the search hook")
				assert.True(t, integration.Healthy(t.Context()), "adapter must use the new token before the search hook")
				tools := integration.Tools()
				require.Len(t, tools, 1)
				assert.Equal(t, mcp.ToolName("notion_notion-search"), tools[0].Name)
			}
			auth := startNotionOAuth(t, ws)
			assert.Equal(t, "default", auth.Get("scope"))
			assert.Equal(t, f.server.URL+"/mcp", auth.Get("resource"))
			assert.Equal(t, "http://localhost:3847/api/remote/notion/oauth/callback", auth.Get("redirect_uri"))
			assert.Equal(t, "notion-client", auth.Get("client_id"))
			assert.Equal(t, "code", auth.Get("response_type"))
			assert.Equal(t, "S256", auth.Get("code_challenge_method"))
			require.NotEmpty(t, auth.Get("state"))
			require.NotEmpty(t, auth.Get("code_challenge"))
			location := notionOAuthCallback(t, ws, url.Values{"code": {"notion-code"}, "state": {auth.Get("state")}})
			assert.Empty(t, location.Query().Get("error"))
			assert.Equal(t, "Connected via MCP OAuth", location.Query().Get("result"))
			assert.Equal(t, 1, hooks)
			_, cached := ws.health.get("notion")
			assert.False(t, cached, "previous health result must be invalidated")
			require.NoError(t, store.Load())
			saved, _ := store.GetIntegration("notion")
			assert.True(t, saved.Enabled)
			assert.Equal(t, initial.ToolGlobs, saved.ToolGlobs)
			assert.Equal(t, f.server.URL, saved.Credentials["mcp_base_url"])
			assert.Equal(t, "legacy-token", saved.Credentials["token_v2"], "sign-in must keep the browser token so the user can switch back")
			assert.Equal(t, "oauth", saved.Credentials[mcp.CredKeyTokenSource])
			assert.Equal(t, "notion-oauth-token", saved.Credentials["mcp_access_token"])
			f.mu.Lock()
			defer f.mu.Unlock()
			assert.GreaterOrEqual(t, f.discoveries, 1)
			assert.Equal(t, "Switchboard", f.registration["client_name"])
			assert.Equal(t, "none", f.registration["token_endpoint_auth_method"])
			assert.Equal(t, []any{auth.Get("redirect_uri")}, f.registration["redirect_uris"])
			assert.Equal(t, []any{"authorization_code", "refresh_token"}, f.registration["grant_types"])
			assert.Equal(t, 1, f.tokenCalls)
			assert.Equal(t, "notion-code", f.tokenForm.Get("code"))
			assert.Equal(t, "notion-client", f.tokenForm.Get("client_id"))
			assert.Equal(t, "authorization_code", f.tokenForm.Get("grant_type"))
			assert.Equal(t, auth.Get("redirect_uri"), f.tokenForm.Get("redirect_uri"))
			assert.Equal(t, auth.Get("resource"), f.tokenForm.Get("resource"))
			assert.Empty(t, f.tokenForm.Get("client_secret"))
			verifier := f.tokenForm.Get("code_verifier")
			assert.GreaterOrEqual(t, len(verifier), 43)
			digest := sha256.Sum256([]byte(verifier))
			assert.Equal(t, auth.Get("code_challenge"), base64.RawURLEncoding.EncodeToString(digest[:]))
		})
	}
}

type notionConfigureSpy struct {
	mockIntegration
	configure func(mcp.Credentials) error
}

func (s *notionConfigureSpy) Configure(_ context.Context, creds mcp.Credentials) error {
	return s.configure(creds)
}

func TestNotionOAuthPersistence(t *testing.T) {
	for _, tc := range []struct {
		name         string
		initial      *mcp.IntegrationConfig
		saveErr      error
		configureErr error
		wantError    string
	}{
		{name: "missing config"},
		{name: "nil credentials", initial: &mcp.IntegrationConfig{Enabled: true}},
		{name: "save failure", initial: &mcp.IntegrationConfig{Credentials: mcp.Credentials{"mcp_access_token": "old-token", "token_v2": "legacy-token"}}, saveErr: errors.New("disk full"), wantError: "save"},
		{name: "configure failure", initial: &mcp.IntegrationConfig{Credentials: mcp.Credentials{"mcp_access_token": "old-token", "token_v2": "legacy-token"}}, configureErr: errors.New("invalid configuration"), wantError: "configure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newNotionOAuthFixture(t, http.StatusOK)
			ws, reg, cfg := setupTestWeb()
			if tc.initial != nil {
				cfg.cfg.Integrations["notion"] = tc.initial
			}
			before, err := json.Marshal(cfg.Get())
			require.NoError(t, err)
			reg.integrations["notion"] = notionHostedAt(t, f.server.URL)
			auth := startNotionOAuth(t, ws)
			cfg.setErr = tc.saveErr
			ws.health.entries["notion"] = healthEntry{Enabled: true, Healthy: true, CheckedAt: time.Now()}
			configured, hooks := 0, 0
			reg.integrations["notion"] = &notionConfigureSpy{mockIntegration: mockIntegration{name: "notion"}, configure: func(creds mcp.Credentials) error {
				configured++
				saved, exists := cfg.GetIntegration("notion")
				require.True(t, exists)
				assert.Equal(t, "notion-oauth-token", saved.Credentials["mcp_access_token"], "save must finish before Configure")
				assert.Equal(t, saved.Credentials, creds)
				return tc.configureErr
			}}
			ws.onConfigChange = func() { hooks++; assert.Equal(t, 1, configured) }
			browserToken := ""
			if tc.initial != nil {
				browserToken = tc.initial.Credentials["token_v2"]
			}
			location := notionOAuthCallback(t, ws, url.Values{"code": {"code"}, "state": {auth.Get("state")}})
			if tc.wantError != "" {
				assert.Contains(t, location.Query().Get("error"), tc.wantError)
				assert.Empty(t, location.Query().Get("result"))
				assert.Zero(t, hooks)
			} else {
				assert.Empty(t, location.Query().Get("error"))
				assert.NotEmpty(t, location.Query().Get("result"))
				assert.Equal(t, 1, hooks)
			}
			if tc.saveErr != nil {
				assert.Zero(t, configured)
				after, err := json.Marshal(cfg.Get())
				require.NoError(t, err)
				assert.JSONEq(t, string(before), string(after), "failed persistence must leave prior config untouched")
			} else {
				assert.Equal(t, 1, configured)
				saved, exists := cfg.GetIntegration("notion")
				require.True(t, exists)
				assert.Equal(t, "notion-oauth-token", saved.Credentials["mcp_access_token"])
				assert.True(t, saved.Enabled)
			}
			_, cached := ws.health.get("notion")
			assert.Equal(t, tc.saveErr != nil, cached, "invalidate stale health after durable saves, even if configuration fails")
			if browserToken != "" {
				saved, _ := cfg.GetIntegration("notion")
				assert.Equal(t, browserToken, saved.Credentials["token_v2"], "sign-in must keep the browser token")
			}
		})
	}
}

func TestNotionOAuthRejection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		query       url.Values
		tokenStatus int
		want        string
	}{
		{name: "denied", query: url.Values{"error": {"access_denied"}}, tokenStatus: http.StatusOK, want: "access_denied"},
		{name: "missing code", query: url.Values{}, tokenStatus: http.StatusOK, want: "No authorization code"},
		{name: "invalid state", query: url.Values{"code": {"code"}, "state": {"wrong-state"}}, tokenStatus: http.StatusOK, want: "invalid state"},
		{name: "token rejected", query: url.Values{"code": {"code"}}, tokenStatus: http.StatusBadRequest, want: "token endpoint"},
		{name: "error query escaped", query: url.Values{"error": {"denied&result=forged#fragment"}}, tokenStatus: http.StatusOK, want: "denied&result=forged#fragment"},
		{name: "error with code", query: url.Values{"error": {"access_denied"}, "code": {"code"}}, tokenStatus: http.StatusOK, want: "access_denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newNotionOAuthFixture(t, tc.tokenStatus)
			ws, reg, cfg := setupTestWeb()
			reg.integrations["notion"] = notionHostedAt(t, f.server.URL)
			auth := startNotionOAuth(t, ws)
			if tc.query.Get("state") == "" {
				tc.query.Set("state", auth.Get("state"))
			}
			hooks := 0
			ws.onConfigChange = func() { hooks++ }
			before, err := json.Marshal(cfg.Get())
			require.NoError(t, err)
			location := notionOAuthCallback(t, ws, tc.query)
			assert.Contains(t, location.Query().Get("error"), tc.want)
			assert.Empty(t, location.Query().Get("result"))
			assert.Empty(t, location.Fragment)
			assert.Zero(t, hooks)
			after, err := json.Marshal(cfg.Get())
			require.NoError(t, err)
			assert.JSONEq(t, string(before), string(after))
			f.mu.Lock()
			defer f.mu.Unlock()
			if tc.tokenStatus == http.StatusOK {
				assert.Zero(t, f.tokenCalls)
			}
		})
	}
}

func notionHostedAt(t *testing.T, baseURL string) mcp.Integration {
	t.Helper()
	integration := notionInt.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"mcp_access_token": "previous-token", "mcp_base_url": baseURL}))
	return integration
}
