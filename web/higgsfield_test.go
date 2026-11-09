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

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/integrations/higgsfield"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHiggsfieldOAuthProfile(t *testing.T) {
	profile, ok := remoteOAuthProfiles["higgsfield"]
	require.True(t, ok)
	assert.Equal(t, "openid email offline_access", profile.Options.Scope)
	assert.Equal(t, "/mcp", profile.ResourcePath)
	assert.Equal(t, "/api/remote/higgsfield/oauth/callback", profile.CallbackPath)
	assert.Equal(t, "mcp_access_token", profile.CredentialKey)
}

func TestHiggsfieldSetup(t *testing.T) {
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
			integration := &mockIntegration{name: "higgsfield"}
			reg.integrations["higgsfield"] = integration
			if tc.ic != nil {
				cfg.cfg.Integrations["higgsfield"] = tc.ic
			}
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/higgsfield/setup", nil))
			require.Equal(t, http.StatusOK, rr.Code)
			body := rr.Body.String()
			for _, text := range []string{"Higgsfield Setup", "Sign in with Higgsfield", "/api/remote/higgsfield/oauth/start", "credits", "Try Again", tc.want} {
				assert.Contains(t, body, text)
			}
			assert.NotContains(t, body, "private-token")
			assert.Nil(t, integration.lastCreds, "setup GET must not reconfigure the adapter")
		})
	}
}

func TestHiggsfieldDetailRedirect(t *testing.T) {
	ws, reg, _ := setupTestWeb()
	reg.integrations["higgsfield"] = higgsfield.New()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/higgsfield", nil))
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations/higgsfield/setup", rr.Header().Get("Location"))
}

// higgsfieldOAuthFixture fakes Higgsfield's OAuth discovery, dynamic client
// registration, token endpoint, and a hosted MCP server exposing balance.
type higgsfieldOAuthFixture struct {
	server    *httptest.Server
	mu        sync.Mutex
	tokenForm url.Values
}

func newHiggsfieldOAuthFixture(t *testing.T) *higgsfieldOAuthFixture {
	t.Helper()
	f := &higgsfieldOAuthFixture{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "higgsfield-fixture", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{Name: "balance", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"credits":10,"subscription_plan_type":"free"}`}}}, nil
	})
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/oauth2/authorize",
			"token_endpoint": f.server.URL + "/oauth2/token", "registration_endpoint": f.server.URL + "/oauth2/register",
			"code_challenge_methods_supported": []string{"S256"}, "scopes_supported": []string{"openid", "email", "offline_access"},
		})
	})
	mux.HandleFunc("POST /oauth2/register", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]string{"client_id": "higgsfield-client"})
	})
	mux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.tokenForm = r.PostForm
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"access_token": "higgsfield-oauth-token", "token_type": "Bearer"})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer higgsfield-oauth-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func TestHiggsfieldOAuthFlow(t *testing.T) {
	f := newHiggsfieldOAuthFixture(t)
	ws, reg, _ := setupTestWeb()
	t.Setenv("HOME", t.TempDir())
	store, err := config.NewManager()
	require.NoError(t, err)
	ws.services.Config = store
	initial := mcp.Credentials{"base_url": f.server.URL, "mcp_access_token": "previous-token"}
	require.NoError(t, store.SetIntegration("higgsfield", &mcp.IntegrationConfig{Credentials: initial}))
	integration := higgsfield.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	require.NoError(t, integration.Configure(t.Context(), initial))
	reg.integrations["higgsfield"] = integration

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/remote/higgsfield/oauth/start", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var started map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &started))
	require.Empty(t, started["error"])
	authorizeURL, err := url.Parse(started["authorize_url"])
	require.NoError(t, err)
	auth := authorizeURL.Query()
	assert.Equal(t, f.server.URL+"/oauth2/authorize", authorizeURL.Scheme+"://"+authorizeURL.Host+authorizeURL.Path)
	assert.Equal(t, "openid email offline_access", auth.Get("scope"))
	assert.Equal(t, f.server.URL+"/mcp", auth.Get("resource"))
	assert.Equal(t, "http://localhost:3847/api/remote/higgsfield/oauth/callback", auth.Get("redirect_uri"))
	assert.Equal(t, "higgsfield-client", auth.Get("client_id"))

	rr = httptest.NewRecorder()
	query := url.Values{"code": {"higgsfield-code"}, "state": {auth.Get("state")}}
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/remote/higgsfield/oauth/callback?"+query.Encode(), nil))
	require.Equal(t, http.StatusSeeOther, rr.Code)
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/integrations/higgsfield/setup", location.Path)
	assert.Equal(t, "Connected via MCP OAuth", location.Query().Get("result"))

	require.NoError(t, store.Load())
	saved, _ := store.GetIntegration("higgsfield")
	assert.True(t, saved.Enabled)
	assert.Equal(t, "higgsfield-oauth-token", saved.Credentials["mcp_access_token"])
	assert.Equal(t, f.server.URL, saved.Credentials["base_url"])
	assert.True(t, integration.Healthy(t.Context()), "adapter must use the new token immediately")
	result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.JSONEq(t, `{"credits":10,"subscription_plan_type":"free"}`, result.Data)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "higgsfield-code", f.tokenForm.Get("code"))
	assert.Equal(t, auth.Get("resource"), f.tokenForm.Get("resource"))
}
