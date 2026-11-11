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
	"github.com/daltoniam/switchboard/integrations/mobbin"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMobbinOAuthProfile(t *testing.T) {
	profile, ok := remoteOAuthProfiles["mobbin"]
	require.True(t, ok)
	assert.Equal(t, "openid", profile.Options.Scope)
	assert.Equal(t, "/mcp", profile.ResourcePath)
	assert.Equal(t, "/api/remote/mobbin/oauth/callback", profile.CallbackPath)
	assert.Equal(t, "mcp_access_token", profile.CredentialKey)
}

func TestMobbinSetup(t *testing.T) {
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
			integration := &mockIntegration{name: "mobbin"}
			reg.integrations["mobbin"] = integration
			if tc.ic != nil {
				cfg.cfg.Integrations["mobbin"] = tc.ic
			}
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/mobbin/setup", nil))
			require.Equal(t, http.StatusOK, rr.Code)
			body := rr.Body.String()
			for _, text := range []string{"Mobbin Setup", "Sign in with Mobbin", "/api/remote/mobbin/oauth/start", "aria-live=\"polite\"", tc.want} {
				assert.Contains(t, body, text)
			}
			assert.NotContains(t, body, "private-token")
			assert.Nil(t, integration.lastCreds, "setup GET must not reconfigure the adapter")
		})
	}
}

func TestMobbinDetailRedirect(t *testing.T) {
	ws, reg, _ := setupTestWeb()
	reg.integrations["mobbin"] = mobbin.New()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/mobbin", nil))
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations/mobbin/setup", rr.Header().Get("Location"))
}

// mobbinOAuthFixture mirrors api.mobbin.com: no origin authorization metadata,
// RFC 9728 resource metadata pointing at a Supabase issuer with a path.
type mobbinOAuthFixture struct {
	server    *httptest.Server
	mu        sync.Mutex
	tokenForm url.Values
}

func newMobbinOAuthFixture(t *testing.T) *mobbinOAuthFixture {
	t.Helper()
	f := &mobbinOAuthFixture{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "mobbin-fixture", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{Name: "list_collections", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"collections":[]}`}}}, nil
	})
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	issuer := func() string { return f.server.URL + "/auth/v1" }
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"resource": f.server.URL + "/mcp", "authorization_servers": []string{issuer()}, "scopes_supported": []string{"openid"}})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server/auth/v1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer": issuer(), "authorization_endpoint": issuer() + "/oauth/authorize",
			"token_endpoint": issuer() + "/oauth/token", "registration_endpoint": issuer() + "/oauth/clients/register",
		})
	})
	mux.HandleFunc("POST /auth/v1/oauth/clients/register", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]string{"client_id": "mobbin-client"})
	})
	mux.HandleFunc("POST /auth/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.tokenForm = r.PostForm
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"access_token": "mobbin-oauth-token", "token_type": "bearer"})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mobbin-oauth-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func TestMobbinOAuthFlow(t *testing.T) {
	f := newMobbinOAuthFixture(t)
	ws, reg, _ := setupTestWeb()
	t.Setenv("HOME", t.TempDir())
	store, err := config.NewManager()
	require.NoError(t, err)
	ws.services.Config = store
	// An expired token from an earlier sign-in; reauthorization replaces it.
	initial := &mcp.IntegrationConfig{Credentials: mcp.Credentials{"base_url": f.server.URL, "mcp_access_token": "expired-token"}}
	require.NoError(t, store.SetIntegration("mobbin", initial))
	integration := mobbin.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	require.NoError(t, integration.Configure(t.Context(), initial.Credentials))
	require.False(t, integration.Healthy(t.Context()))
	reg.integrations["mobbin"] = integration

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/remote/mobbin/oauth/start", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var started map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &started))
	require.Empty(t, started["error"])
	authorizeURL, err := url.Parse(started["authorize_url"])
	require.NoError(t, err)
	assert.Equal(t, f.server.URL+"/auth/v1/oauth/authorize", authorizeURL.Scheme+"://"+authorizeURL.Host+authorizeURL.Path)
	auth := authorizeURL.Query()
	assert.Equal(t, "openid", auth.Get("scope"))
	assert.Equal(t, f.server.URL+"/mcp", auth.Get("resource"))
	assert.Equal(t, "http://localhost:3847/api/remote/mobbin/oauth/callback", auth.Get("redirect_uri"))
	assert.Equal(t, "mobbin-client", auth.Get("client_id"))

	rr = httptest.NewRecorder()
	callback := url.Values{"code": {"mobbin-code"}, "state": {auth.Get("state")}}
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/remote/mobbin/oauth/callback?"+callback.Encode(), nil))
	require.Equal(t, http.StatusSeeOther, rr.Code)
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/integrations/mobbin/setup", location.Path)
	assert.Equal(t, "Connected via MCP OAuth", location.Query().Get("result"))

	require.NoError(t, store.Load())
	saved, _ := store.GetIntegration("mobbin")
	assert.True(t, saved.Enabled)
	assert.Equal(t, "mobbin-oauth-token", saved.Credentials["mcp_access_token"])
	assert.Equal(t, "oauth", saved.Credentials[mcp.CredKeyTokenSource])
	assert.True(t, integration.Healthy(t.Context()), "adapter must use the new token without a restart")
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "mobbin-code", f.tokenForm.Get("code"))
	assert.Equal(t, f.server.URL+"/mcp", f.tokenForm.Get("resource"))
}
