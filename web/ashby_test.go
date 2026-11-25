package web

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	assert.Equal(t, "api_key", profile.ClearCredentialKey, "signing in must leave API key mode")
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
		{name: "enabled with api key", ic: &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"api_key": "private-token"}}, want: "API Key Saved"},
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
			for _, text := range []string{"Ashby Setup", "Sign in with Ashby", "/api/remote/ashby/oauth/start", "Opt-In Features", "Elevated Access", "Try Again", "Option 2: API key", `action="/integrations/ashby/api-key"`, `id="cred_api_key"`, "https://app.ashbyhq.com/admin/api/credentials", "Hiring Process", "confidential jobs and projects", tc.want} {
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
	initial := mcp.Credentials{"base_url": f.server.URL, "api_key": "previous-api-key"}
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
	assert.Empty(t, saved.Credentials["api_key"], "signing in replaces API key mode")
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

func TestAshbySetup_ActiveOption(t *testing.T) {
	for _, tc := range []struct {
		name  string
		creds mcp.Credentials
		want  string
		retry string
	}{
		{name: "sign-in", creds: mcp.Credentials{"mcp_access_token": "t"}, want: "Active option: <strong>Sign in with Ashby</strong>", retry: "Re-authorize with Ashby"},
		{name: "api key", creds: mcp.Credentials{"api_key": "k"}, want: "Active option: <strong>API key</strong>", retry: "Sign in with Ashby →"},
		{name: "api key wins", creds: mcp.Credentials{"api_key": "k", "mcp_access_token": "t"}, want: "Active option: <strong>API key</strong>", retry: "Sign in with Ashby →"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			reg.integrations["ashby"] = &mockIntegration{name: "ashby"}
			cfg.cfg.Integrations["ashby"] = &mcp.IntegrationConfig{Enabled: true, Credentials: tc.creds}
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/ashby/setup", nil))
			body := rr.Body.String()
			assert.Contains(t, body, tc.want)
			assert.Contains(t, body, tc.retry)
			assert.Equal(t, 1, strings.Count(body, ">Active</span>"), "exactly one option is marked active")
			assert.NotContains(t, body, `value="k"`, "the saved key is never rendered")
		})
	}
}

// ashbyAPIFixture fakes Ashby's REST API: Basic auth with the key as the
// username, 403 for any other key, and fixed records.
func ashbyAPIFixture(t *testing.T, key string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok || user == "" {
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}
		if user != key {
			http.Error(w, "Invalid or deactivated API key", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apiKey.info":
			_, _ = io.WriteString(w, `{"success":true,"results":{"title":"Fixture key","createdAt":"2026-01-01T00:00:00.000Z"}}`)
		case "/candidate.search":
			_, _ = io.WriteString(w, `{"success":true,"results":[{"id":"cand-fixture","name":"Fixture Candidate","createdAt":"2026-01-01T00:00:00.000Z","updatedAt":"2026-01-02T00:00:00.000Z","primaryEmailAddress":{"value":"fixture@example.com"},"primaryPhoneNumber":{"value":"+1 555 0100"},"applicationIds":["app-fixture"]}],"moreDataAvailable":false}`)
		default:
			_, _ = io.WriteString(w, `{"success":false,"errorInfo":{"code":"unexpected_endpoint","message":"`+r.URL.Path+`"}}`)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAshbyAPIKeySave(t *testing.T) {
	api := ashbyAPIFixture(t, "good-key")
	signedIn := mcp.Credentials{
		"api_base_url":         api.URL,
		"mcp_access_token":     "oauth-token",
		"mcp_refresh_token":    "refresh",
		"mcp_client_id":        "client",
		"mcp_token_expiry":     "2030-01-01T00:00:00Z",
		mcp.CredKeyTokenSource: "oauth",
	}
	for _, tc := range []struct {
		name        string
		creds       mcp.Credentials
		form        url.Values
		header      map[string]string
		wantStatus  int
		wantFlash   string
		wantKey     string
		wantEnabled bool
		wantHealthy bool
		unchanged   bool
	}{
		{name: "key required", creds: mcp.Credentials{}, form: url.Values{"enabled": {"true"}}, wantStatus: http.StatusSeeOther, wantFlash: "error=API+key+is+required", unchanged: true},
		{name: "switches a sign-in to the API key", creds: signedIn, form: url.Values{"enabled": {"true"}, "cred_api_key": {" good-key "}}, wantStatus: http.StatusSeeOther, wantFlash: "result=Configuration+saved", wantKey: "good-key", wantEnabled: true, wantHealthy: true},
		{name: "blank key keeps the saved key", creds: mcp.Credentials{"api_base_url": api.URL, "api_key": "good-key"}, form: url.Values{"cred_api_key": {""}}, wantStatus: http.StatusSeeOther, wantFlash: "result=Configuration+saved", wantKey: "good-key"},
		{name: "rejected key saves but is unhealthy", creds: mcp.Credentials{"api_base_url": api.URL}, form: url.Values{"enabled": {"true"}, "cred_api_key": {"bad-key"}}, wantStatus: http.StatusSeeOther, wantFlash: "result=Configuration+saved", wantKey: "bad-key", wantEnabled: true},
		{name: "cross-site post refused", creds: mcp.Credentials{}, form: url.Values{"cred_api_key": {"good-key"}}, header: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantStatus: http.StatusForbidden, unchanged: true},
		{name: "foreign origin refused without fetch metadata", creds: mcp.Credentials{}, form: url.Values{"cred_api_key": {"good-key"}}, header: map[string]string{"Origin": "https://attacker.test"}, wantStatus: http.StatusForbidden, unchanged: true},
		{name: "opaque origin refused", creds: mcp.Credentials{}, form: url.Values{"cred_api_key": {"good-key"}}, header: map[string]string{"Origin": "null"}, wantStatus: http.StatusForbidden, unchanged: true},
		{name: "foreign referer refused without origin", creds: mcp.Credentials{}, form: url.Values{"cred_api_key": {"good-key"}}, header: map[string]string{"Referer": "https://attacker.test/form"}, wantStatus: http.StatusForbidden, unchanged: true},
		{name: "http origin refused for an https app", creds: mcp.Credentials{}, form: url.Values{"cred_api_key": {"good-key"}}, header: map[string]string{"Origin": "http://example.com", "X-Forwarded-Proto": "https"}, wantStatus: http.StatusForbidden, unchanged: true},
		{name: "https referer refused for an http app", creds: mcp.Credentials{}, form: url.Values{"cred_api_key": {"good-key"}}, header: map[string]string{"Referer": "https://example.com/integrations/ashby/setup"}, wantStatus: http.StatusForbidden, unchanged: true},
		{name: "same origin accepted", creds: mcp.Credentials{"api_base_url": api.URL}, form: url.Values{"enabled": {"true"}, "cred_api_key": {"good-key"}}, header: map[string]string{"Origin": "http://example.com"}, wantStatus: http.StatusSeeOther, wantFlash: "result=Configuration+saved", wantKey: "good-key", wantEnabled: true, wantHealthy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			integration := ashby.New()
			t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
			reg.integrations["ashby"] = integration
			before := maps.Clone(tc.creds)
			cfg.cfg.Integrations["ashby"] = &mcp.IntegrationConfig{Credentials: maps.Clone(tc.creds)}
			req := httptest.NewRequest(http.MethodPost, "/integrations/ashby/api-key", strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, req)
			require.Equal(t, tc.wantStatus, rr.Code, rr.Body.String())
			saved := cfg.cfg.Integrations["ashby"]
			if tc.unchanged {
				assert.Equal(t, before, saved.Credentials)
				assert.False(t, saved.Enabled)
				if tc.wantFlash != "" {
					assert.Contains(t, rr.Header().Get("Location"), tc.wantFlash)
				}
				return
			}
			assert.Equal(t, "/integrations/ashby/setup?"+tc.wantFlash, rr.Header().Get("Location"))
			assert.Equal(t, tc.wantKey, saved.Credentials["api_key"])
			assert.Equal(t, tc.wantEnabled, saved.Enabled)
			assert.Equal(t, api.URL, saved.Credentials["api_base_url"], "other settings are kept")
			assert.Equal(t, "api_key", saved.Credentials[mcp.CredKeyTokenSource])
			for _, key := range []string{"mcp_access_token", "mcp_refresh_token", "mcp_client_id", "mcp_token_expiry"} {
				assert.Empty(t, saved.Credentials[key], "%s must be cleared so only API key mode remains", key)
			}
			entry, ok := ws.health.get("ashby")
			require.True(t, ok)
			assert.Equal(t, tc.wantEnabled, entry.Enabled)
			assert.Equal(t, tc.wantHealthy, entry.Healthy)

			rr = httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/ashby/setup", nil))
			body := rr.Body.String()
			switch {
			case !tc.wantEnabled:
				assert.Contains(t, body, ">Disabled</span>")
			case tc.wantHealthy:
				assert.Contains(t, body, ">Connected</span>")
			default:
				assert.Contains(t, body, ">Connection Error</span>")
			}
			assert.NotContains(t, body, tc.wantKey)
		})
	}
}

func TestAshbyAPIKeySave_ToolsAndExecute(t *testing.T) {
	api := ashbyAPIFixture(t, "good-key")
	ws, reg, cfg := setupTestWeb()
	integration := ashby.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	reg.integrations["ashby"] = integration
	cfg.cfg.Integrations["ashby"] = &mcp.IntegrationConfig{Credentials: mcp.Credentials{"api_base_url": api.URL}}
	req := httptest.NewRequest(http.MethodPost, "/integrations/ashby/api-key", strings.NewReader(url.Values{"enabled": {"true"}, "cred_api_key": {"good-key"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ws.Handler().ServeHTTP(httptest.NewRecorder(), req)

	names := map[mcp.ToolName]bool{}
	for _, tool := range integration.Tools() {
		names[tool.Name] = true
	}
	assert.True(t, names["ashby_search_candidates"])
	assert.False(t, names["ashby_get_pending_tasks"], "hosted-only tools leave the catalog in API key mode")
	result, err := integration.Execute(t.Context(), "ashby_search_candidates", map[string]any{"name": "Fixture"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, "Fixture Candidate")
}
