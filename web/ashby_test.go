package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/integrations/ashby"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAshbyOAuthProfile(t *testing.T) {
	profile, ok := remoteOAuthProfiles["ashby"]
	require.True(t, ok)
	assert.Equal(t, "openid mcp offline_access", profile.Options.Scope)
	assert.Equal(t, "consent", profile.Options.Prompt)
	assert.Equal(t, "/mcp/v1", profile.ResourcePath)
	assert.Equal(t, "/api/remote/ashby/oauth/callback", profile.CallbackPath)
	assert.Equal(t, "mcp_access_token", profile.CredentialKey)
	assert.True(t, profile.Renewable)
}

func TestAshbySetup(t *testing.T) {
	for _, tc := range []struct {
		name string
		ic   *mcp.IntegrationConfig
		want string
	}{
		{name: "missing config", want: "Not Connected"},
		{name: "disabled with token", ic: &mcp.IntegrationConfig{Credentials: mcp.Credentials{"mcp_access_token": "private-token"}}, want: "Disabled"},
		{name: "enabled with token", ic: &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"mcp_access_token": "private-token"}}, want: "Token Saved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			integration := &mockIntegration{name: "ashby"}
			reg.integrations["ashby"] = integration
			if tc.ic != nil {
				cfg.cfg.Integrations["ashby"] = tc.ic
			}
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/ashby/setup", nil))
			require.Equal(t, http.StatusOK, rr.Code)
			body := rr.Body.String()
			for _, text := range []string{"Ashby Setup", "Sign in with Ashby", "/api/remote/ashby/oauth/start", "Opt-In Features", "Elevated Access", "Try Again", tc.want} {
				assert.Contains(t, body, text)
			}
			assert.NotContains(t, body, "private-token")
			assert.Nil(t, integration.lastCreds, "setup GET must not reconfigure the adapter")
		})
	}
}

func TestAshbyDetailRedirect(t *testing.T) {
	ws, reg, _ := setupTestWeb()
	reg.integrations["ashby"] = ashby.New()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/ashby", nil))
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations/ashby/setup", rr.Header().Get("Location"))
}

// ashbyOAuthFixture fakes Ashby's hosted MCP sign-in: protected resource
// metadata at the origin naming the /mcp/v1 endpoint, an OpenID issuer under
// /oidc with dynamic client registration, and the MCP server at /mcp/v1.
type ashbyOAuthFixture struct {
	server     *httptest.Server
	mu         sync.Mutex
	tokenForm  url.Values
	registered map[string]any
}

func newAshbyOAuthFixture(t *testing.T) *ashbyOAuthFixture {
	t.Helper()
	f := &ashbyOAuthFixture{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "ashby-fixture", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{Name: "get_pending_tasks", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"tasks":[{"type":"scorecard","candidate":"Fixture Candidate"}]}`}}}, nil
	})
	server.AddTool(&mcpsdk.Tool{Name: "search_records_by_name", InputSchema: map[string]any{
		"type":       "object",
		"properties": map[string]any{"query": map[string]any{"type": "string", "description": "Name to resolve"}},
	}}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"results":[{"type":"candidate","id":"cand-fixture","name":"Fixture Candidate"}]}`}}}, nil
	})
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"resource": f.server.URL + "/mcp/v1", "authorization_servers": []string{f.server.URL + "/oidc"}})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server/oidc", func(w http.ResponseWriter, _ *http.Request) {
		issuer := f.server.URL + "/oidc"
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer": issuer, "authorization_endpoint": issuer + "/auth",
			"token_endpoint": issuer + "/token", "registration_endpoint": issuer + "/reg",
			"code_challenge_methods_supported": []string{"S256"}, "scopes_supported": []string{"openid", "mcp", "offline_access"},
		})
	})
	mux.HandleFunc("POST /oidc/reg", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.registered = body
		f.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]string{"client_id": "ashby-client"})
	})
	mux.HandleFunc("POST /oidc/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.tokenForm = r.PostForm
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "ashby-oauth-token", "token_type": "Bearer", "refresh_token": "ashby-refresh-token", "expires_in": 3600})
	})
	mux.HandleFunc("/mcp/v1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ashby-oauth-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func TestAshbyOAuthFlow(t *testing.T) {
	f := newAshbyOAuthFixture(t)
	ws, reg, _ := setupTestWeb()
	t.Setenv("HOME", t.TempDir())
	store, err := config.NewManager()
	require.NoError(t, err)
	ws.services.Config = store
	initial := mcp.Credentials{"base_url": f.server.URL, "mcp_access_token": "previous-token"}
	require.NoError(t, store.SetIntegration("ashby", &mcp.IntegrationConfig{Credentials: initial}))
	integration := ashby.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	require.NoError(t, integration.Configure(t.Context(), initial))
	reg.integrations["ashby"] = integration

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/remote/ashby/oauth/start", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var started map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &started))
	require.Empty(t, started["error"])
	authorizeURL, err := url.Parse(started["authorize_url"])
	require.NoError(t, err)
	auth := authorizeURL.Query()
	assert.Equal(t, f.server.URL+"/oidc/auth", authorizeURL.Scheme+"://"+authorizeURL.Host+authorizeURL.Path)
	assert.Equal(t, "openid mcp offline_access", auth.Get("scope"))
	assert.Equal(t, "consent", auth.Get("prompt"))
	assert.Equal(t, f.server.URL+"/mcp/v1", auth.Get("resource"))
	assert.Equal(t, "S256", auth.Get("code_challenge_method"))
	assert.Equal(t, "http://localhost:3847/api/remote/ashby/oauth/callback", auth.Get("redirect_uri"))
	assert.Equal(t, "ashby-client", auth.Get("client_id"))
	f.mu.Lock()
	assert.Equal(t, []any{"http://localhost:3847/api/remote/ashby/oauth/callback"}, f.registered["redirect_uris"])
	f.mu.Unlock()

	rr = httptest.NewRecorder()
	query := url.Values{"code": {"ashby-code"}, "state": {auth.Get("state")}}
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/remote/ashby/oauth/callback?"+query.Encode(), nil))
	require.Equal(t, http.StatusSeeOther, rr.Code)
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/integrations/ashby/setup", location.Path)
	assert.Equal(t, "Connected via MCP OAuth", location.Query().Get("result"))

	require.NoError(t, store.Load())
	saved, _ := store.GetIntegration("ashby")
	assert.True(t, saved.Enabled)
	assert.Equal(t, "ashby-oauth-token", saved.Credentials["mcp_access_token"])
	assert.Equal(t, f.server.URL, saved.Credentials["base_url"])
	assert.Equal(t, "ashby-refresh-token", saved.Credentials["mcp_refresh_token"], "the refresh token is kept so the login can be renewed")
	assert.Equal(t, "ashby-client", saved.Credentials["mcp_client_id"])
	expiry, err := time.Parse(time.RFC3339, saved.Credentials["mcp_token_expiry"])
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(time.Hour), expiry, time.Minute)
	assert.True(t, integration.Healthy(t.Context()), "adapter must use the new token immediately")
	result, err := integration.Execute(t.Context(), "ashby_get_pending_tasks", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.JSONEq(t, `{"tasks":[{"type":"scorecard","candidate":"Fixture Candidate"}]}`, result.Data)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "ashby-code", f.tokenForm.Get("code"))
	assert.Equal(t, auth.Get("resource"), f.tokenForm.Get("resource"))
	assert.NotEmpty(t, f.tokenForm.Get("code_verifier"))
}

func TestAshbyOAuthStartReportsDiscoveryFailure(t *testing.T) {
	broken := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(broken.Close)
	ws, reg, _ := setupTestWeb()
	integration := ashby.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": broken.URL, "mcp_access_token": "t"}))
	reg.integrations["ashby"] = integration

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/remote/ashby/oauth/start", nil))
	var started map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &started))
	assert.Contains(t, started["error"], "oauth discovery returned 404")
	assert.Empty(t, started["authorize_url"])
}
