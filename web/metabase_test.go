package web

import (
	"context"
	"encoding/json"
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
	"github.com/daltoniam/switchboard/integrations/metabase"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetabaseOAuthProfile(t *testing.T) {
	profile, ok := remoteOAuthProfiles["metabase"]
	require.True(t, ok)
	assert.Equal(t, "/api/remote/metabase/oauth/callback", profile.CallbackPath)
	assert.Equal(t, metabase.MCPEndpointPath, profile.ResourcePath)
	assert.Equal(t, "agent:content:read agent:content:write agent:query:run agent:sql:run agent:delivery:write agent:resource:read", profile.Options.Scope)
	assert.Equal(t, "mcp_access_token", profile.CredentialKey)
	assert.Empty(t, profile.ClearCredentialKey, "the API key stays as the fallback")
	assert.True(t, profile.PersistRefresh)
	assert.True(t, setupIntegrations["metabase"])
}

func TestMetabaseDetailRedirect(t *testing.T) {
	ws, reg, _ := setupTestWeb()
	reg.integrations["metabase"] = metabase.New()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/metabase", nil))
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations/metabase/setup", rr.Header().Get("Location"))
}

func TestMetabaseSetup(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ic       *mcp.IntegrationConfig
		want     []string
		wantNot  []string
		probeOff bool
	}{
		{
			name:    "no config",
			want:    []string{"Not Connected", "Save the Metabase URL", "https://your-metabase-instance.com"},
			wantNot: []string{"/api/remote/metabase/oauth/start"},
		},
		{
			name:    "url only, instance has MCP enabled",
			ic:      &mcp.IntegrationConfig{Credentials: mcp.Credentials{"url": "PROBE"}},
			want:    []string{"Sign in with Metabase", "/api/remote/metabase/oauth/start", "MCP server is enabled", "Not Connected"},
			wantNot: []string{"Admin &gt; AI &gt; MCP"},
		},
		{
			name:     "url only, instance has MCP disabled",
			ic:       &mcp.IntegrationConfig{Credentials: mcp.Credentials{"url": "PROBE"}},
			probeOff: true,
			want:     []string{"MCP server is turned off", "Admin &gt; AI &gt; MCP", "API key"},
			wantNot:  []string{"/api/remote/metabase/oauth/start"},
		},
		{
			name: "oauth connected",
			ic:   &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"url": "PROBE", "mcp_access_token": "private-token", "mcp_refresh_token": "private-refresh", mcp.CredKeyTokenSource: "oauth"}},
			want: []string{"Re-authorize", "Signed in via OAuth", "Connected"},
		},
		{
			name: "api key connected",
			ic:   &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"url": "PROBE", "api_key": "private-key", mcp.CredKeyTokenSource: "api_key"}},
			want: []string{"Using API key", "Connected"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMetabaseFixture(t)
			f.setMCPEnabled(!tc.probeOff)
			ws, reg, cfg := setupTestWeb()
			integration := &mockIntegration{name: "metabase", healthy: true}
			reg.integrations["metabase"] = integration
			if tc.ic != nil {
				if tc.ic.Credentials["url"] == "PROBE" {
					tc.ic.Credentials["url"] = f.server.URL
				}
				cfg.cfg.Integrations["metabase"] = tc.ic
				if tc.ic.Enabled {
					ws.health.entries["metabase"] = healthEntry{Enabled: true, Healthy: true, CheckedAt: time.Now()}
				}
			}
			before, err := json.Marshal(cfg.Get())
			require.NoError(t, err)
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/metabase/setup", nil))
			require.Equal(t, http.StatusOK, rr.Code)
			body := rr.Body.String()
			for _, text := range append([]string{"Metabase Setup", "/api/metabase/save-credentials"}, tc.want...) {
				assert.Contains(t, body, text)
			}
			for _, text := range tc.wantNot {
				assert.NotContains(t, body, text)
			}
			assert.NotContains(t, body, "private-token")
			assert.NotContains(t, body, "private-refresh")
			assert.NotContains(t, body, "private-key")
			assert.Nil(t, integration.lastCreds, "setup GET must not reconfigure the adapter")
			after, err := json.Marshal(cfg.Get())
			require.NoError(t, err)
			assert.JSONEq(t, string(before), string(after))
		})
	}
}

func TestMetabaseSaveCredentials_URLOnlyAgainstRealAdapter(t *testing.T) {
	ws, reg, cfg := setupTestWeb()
	reg.integrations["metabase"] = metabase.New()
	req := httptest.NewRequest(http.MethodPost, "/api/metabase/save-credentials", strings.NewReader(url.Values{"url": {"https://mb.example.com"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations/metabase/setup?result=Metabase+URL+saved", rr.Header().Get("Location"))
	saved, ok := cfg.GetIntegration("metabase")
	require.True(t, ok)
	assert.Equal(t, "https://mb.example.com", saved.Credentials["url"])
	assert.False(t, saved.Enabled)
}

func TestMetabaseSetupFlashes(t *testing.T) {
	ws, reg, _ := setupTestWeb()
	reg.integrations["metabase"] = &mockIntegration{name: "metabase"}
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/metabase/setup?result=FORGED&error=%3Cscript%3Ealert(1)%3C/script%3E", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.NotContains(t, rr.Body.String(), "FORGED")
}

func TestMetabaseSaveCredentials(t *testing.T) {
	for _, tc := range []struct {
		name      string
		initial   *mcp.IntegrationConfig
		form      url.Values
		wantQuery string
		wantCreds mcp.Credentials
		wantOn    bool
	}{
		{
			name:      "url required",
			form:      url.Values{"api_key": {"k"}},
			wantQuery: "error=Metabase+URL+is+required",
		},
		{
			name:      "url must be http(s)",
			form:      url.Values{"url": {"ftp://mb"}},
			wantQuery: "error=Metabase+URL+must+start+with+http+or+https",
		},
		{
			name:      "url only is saved without enabling",
			form:      url.Values{"url": {"https://mb.example.com/"}},
			wantQuery: "result=Metabase+URL+saved",
			wantCreds: mcp.Credentials{"url": "https://mb.example.com"},
		},
		{
			name:      "api key enables and selects api_key source",
			initial:   &mcp.IntegrationConfig{Credentials: mcp.Credentials{"url": "https://mb.example.com", "mcp_access_token": "oauth-token", "mcp_refresh_token": "r", "mcp_client_id": "c", mcp.CredKeyTokenSource: "oauth"}, ToolGlobs: []string{"metabase_*"}},
			form:      url.Values{"url": {"https://mb.example.com/"}, "api_key": {" mb_key "}},
			wantQuery: "result=API+key+saved",
			wantCreds: mcp.Credentials{"url": "https://mb.example.com", "api_key": "mb_key", "mcp_access_token": "oauth-token", "mcp_refresh_token": "r", "mcp_client_id": "c", mcp.CredKeyTokenSource: "api_key"},
			wantOn:    true,
		},
		{
			name:      "url change keeps existing api key",
			initial:   &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"url": "https://old", "api_key": "mb_key", mcp.CredKeyTokenSource: "api_key"}},
			form:      url.Values{"url": {"https://new.example.com"}},
			wantQuery: "result=Metabase+URL+saved",
			wantCreds: mcp.Credentials{"url": "https://new.example.com", "api_key": "mb_key", mcp.CredKeyTokenSource: "api_key"},
			wantOn:    true,
		},
		{
			name:      "url change discards oauth tokens issued by the old host",
			initial:   &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"url": "https://old", "api_key": "mb_key", "mcp_access_token": "t", "mcp_refresh_token": "r", "mcp_client_id": "c", mcp.CredKeyTokenSource: "oauth"}},
			form:      url.Values{"url": {"https://new.example.com"}},
			wantQuery: "result=Metabase+URL+saved%3B+sign+in+again+to+use+OAuth+with+the+new+instance",
			wantCreds: mcp.Credentials{"url": "https://new.example.com", "api_key": "mb_key", "mcp_access_token": "", "mcp_refresh_token": "", "mcp_client_id": "", mcp.CredKeyTokenSource: "api_key"},
			wantOn:    true,
		},
		{
			name:      "url change without api key leaves nothing usable",
			initial:   &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"url": "https://old", "mcp_access_token": "t", "mcp_refresh_token": "r", "mcp_client_id": "c", mcp.CredKeyTokenSource: "oauth"}},
			form:      url.Values{"url": {"https://new.example.com"}},
			wantQuery: "result=Metabase+URL+saved%3B+sign+in+again+to+use+OAuth+with+the+new+instance",
			wantCreds: mcp.Credentials{"url": "https://new.example.com", "mcp_access_token": "", "mcp_refresh_token": "", "mcp_client_id": "", mcp.CredKeyTokenSource: ""},
			wantOn:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			integration := &mockIntegration{name: "metabase"}
			reg.integrations["metabase"] = integration
			if tc.initial != nil {
				cfg.cfg.Integrations["metabase"] = tc.initial
			}
			hooks := 0
			ws.onConfigChange = func() { hooks++ }
			req := httptest.NewRequest(http.MethodPost, "/api/metabase/save-credentials", strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, req)
			require.Equal(t, http.StatusSeeOther, rr.Code)
			assert.Equal(t, "/integrations/metabase/setup?"+tc.wantQuery, rr.Header().Get("Location"))
			if tc.wantCreds == nil {
				assert.Equal(t, 0, hooks)
				return
			}
			saved, ok := cfg.GetIntegration("metabase")
			require.True(t, ok)
			assert.Equal(t, tc.wantCreds, saved.Credentials)
			assert.Equal(t, tc.wantOn, saved.Enabled)
			if tc.initial != nil {
				assert.Equal(t, tc.initial.ToolGlobs, saved.ToolGlobs)
			}
			assert.Equal(t, 1, hooks)
		})
	}
}

type metabaseFixture struct {
	server       *httptest.Server
	mu           sync.Mutex
	mcpEnabled   bool
	registration map[string]any
	tokenForm    url.Values
	toolCalls    int
}

func (f *metabaseFixture) setMCPEnabled(on bool) {
	f.mu.Lock()
	f.mcpEnabled = on
	f.mu.Unlock()
}

func newMetabaseFixture(t *testing.T) *metabaseFixture {
	t.Helper()
	f := &metabaseFixture{mcpEnabled: true}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "metabase-fixture", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{Name: "search", Description: "Find content", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		f.mu.Lock()
		f.toolCalls++
		f.mu.Unlock()
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"items":[]}`}}}, nil
	})
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/session/properties", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"mcp-enabled?": f.mcpEnabled, "site-name": "Fixture"})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/oauth/authorize",
			"token_endpoint": f.server.URL + "/oauth/token", "registration_endpoint": f.server.URL + "/oauth/register",
			"code_challenge_methods_supported": []string{"S256"},
		})
	})
	mux.HandleFunc("POST /oauth/register", func(w http.ResponseWriter, r *http.Request) {
		var registration map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&registration))
		f.mu.Lock()
		f.registration = registration
		f.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]string{"client_id": "metabase-client"})
	})
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		f.mu.Lock()
		f.tokenForm = r.PostForm
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "metabase-oauth-token", "refresh_token": "metabase-refresh-token", "token_type": "Bearer", "expires_in": 3600})
	})
	mux.HandleFunc("/api/metabase-mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer metabase-oauth-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func TestMetabaseOAuthFlow(t *testing.T) {
	f := newMetabaseFixture(t)
	ws, reg, _ := setupTestWeb()
	t.Setenv("HOME", t.TempDir())
	store, err := config.NewManager()
	require.NoError(t, err)
	ws.services.Config = store
	initial := &mcp.IntegrationConfig{Enabled: false, Credentials: mcp.Credentials{"url": f.server.URL, "api_key": "mb_key", mcp.CredKeyTokenSource: "api_key"}, ToolGlobs: []string{"metabase_*"}}
	require.NoError(t, store.SetIntegration("metabase", initial))
	integration := metabase.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	require.NoError(t, integration.Configure(t.Context(), initial.Credentials))
	reg.integrations["metabase"] = integration
	ws.health.entries["metabase"] = healthEntry{Enabled: true, Healthy: true, CheckedAt: time.Now()}
	hooks := 0
	ws.onConfigChange = func() {
		hooks++
		assert.True(t, metabase.IsRemoteMCP(integration), "adapter must switch to OAuth mode before the search hook")
		tools := integration.Tools()
		require.Len(t, tools, 1)
		assert.Equal(t, mcp.ToolName("metabase_search"), tools[0].Name)
	}

	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/remote/metabase/oauth/start", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var response map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	require.Empty(t, response["error"])
	authorizeURL, err := url.Parse(response["authorize_url"])
	require.NoError(t, err)
	auth := authorizeURL.Query()
	assert.Equal(t, f.server.URL+"/oauth/authorize", strings.SplitN(response["authorize_url"], "?", 2)[0])
	assert.Equal(t, remoteOAuthProfiles["metabase"].Options.Scope, auth.Get("scope"))
	assert.Equal(t, f.server.URL+"/api/metabase-mcp", auth.Get("resource"))
	assert.Equal(t, "http://localhost:3847/api/remote/metabase/oauth/callback", auth.Get("redirect_uri"))
	assert.Equal(t, "metabase-client", auth.Get("client_id"))
	assert.Equal(t, "S256", auth.Get("code_challenge_method"))

	rr = httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/remote/metabase/oauth/callback?"+url.Values{"code": {"mb-code"}, "state": {auth.Get("state")}}.Encode(), nil))
	require.Equal(t, http.StatusSeeOther, rr.Code)
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/integrations/metabase/setup", location.Path)
	assert.Empty(t, location.Query().Get("error"))
	assert.Equal(t, "Connected via MCP OAuth", location.Query().Get("result"))
	assert.Equal(t, 1, hooks)

	require.NoError(t, store.Load())
	saved, _ := store.GetIntegration("metabase")
	assert.True(t, saved.Enabled)
	assert.Equal(t, initial.ToolGlobs, saved.ToolGlobs)
	assert.Equal(t, mcp.Credentials{
		"url": f.server.URL, "api_key": "mb_key",
		"mcp_access_token": "metabase-oauth-token", "mcp_refresh_token": "metabase-refresh-token", "mcp_client_id": "metabase-client",
		mcp.CredKeyTokenSource: "oauth",
	}, saved.Credentials)

	result, err := integration.Execute(t.Context(), "metabase_search", map[string]any{"q": "x"})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, 1, f.toolCalls)
	assert.Equal(t, "Switchboard", f.registration["client_name"])
	assert.Equal(t, "none", f.registration["token_endpoint_auth_method"])
	assert.Equal(t, "authorization_code", f.tokenForm.Get("grant_type"))
	assert.Equal(t, auth.Get("resource"), f.tokenForm.Get("resource"))
}

func TestMetabaseOAuthStart_RequiresURL(t *testing.T) {
	ws, reg, _ := setupTestWeb()
	reg.integrations["metabase"] = metabase.New()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/remote/metabase/oauth/start", nil))
	var response map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	assert.Contains(t, response["error"], "Metabase URL")
}
