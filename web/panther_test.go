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
	"github.com/daltoniam/switchboard/integrations/panther"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPantherOAuthProfile(t *testing.T) {
	profile, ok := remoteOAuthProfiles["panther"]
	require.True(t, ok)
	assert.True(t, profile.Options.AdvertisedScope, "Panther documents no scope strings")
	assert.Equal(t, "/api/remote/panther/oauth/callback", profile.CallbackPath)
	assert.Equal(t, "mcp_access_token", profile.CredentialKey)
	assert.Equal(t, "api_token", profile.ClearCredentialKey, "signing in must leave API token mode")
	assert.True(t, profile.Renewable)
}

func TestPantherSetup(t *testing.T) {
	for _, tc := range []struct {
		name string
		ic   *mcp.IntegrationConfig
		want []string
	}{
		{name: "missing config", want: []string{"Not Connected", "Save your instance URL first.", "disabled"}},
		{name: "disabled with token", ic: &mcp.IntegrationConfig{Credentials: mcp.Credentials{"instance_url": "https://acme.runpanther.net", "mcp_access_token": "private-token"}}, want: []string{"Disabled", `value="https://acme.runpanther.net"`}},
		{name: "enabled with sign-in", ic: &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"instance_url": "https://acme.runpanther.net", "mcp_access_token": "private-token"}}, want: []string{"Token Saved", "Active option: <strong>Sign in with Panther</strong>", "Re-authorize with Panther"}},
		{name: "enabled with api token", ic: &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"instance_url": "https://acme.runpanther.net", "api_token": "private-token", "mcp_access_token": "private-token"}}, want: []string{"API Token Saved", "Active option: <strong>API token</strong>", "Sign in with Panther →"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			integration := &mockIntegration{name: "panther"}
			reg.integrations["panther"] = integration
			if tc.ic != nil {
				cfg.cfg.Integrations["panther"] = tc.ic
			}
			rr := httptest.NewRecorder()
			ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/panther/setup", nil))
			require.Equal(t, http.StatusOK, rr.Code)
			body := rr.Body.String()
			for _, text := range []string{"Panther Setup", "Your Panther instance", `id="cred_instance_url"`, `action="/integrations/panther/instance"`, "Option 1: Sign in with Panther", "/api/remote/panther/oauth/start", "1.124", "api.", "Option 2: API token", `action="/integrations/panther/api-token"`, `id="cred_api_token"`, "Settings → Developer Tools → API Tokens", "Query Data Lake", "Try Again"} {
				assert.Contains(t, body, text)
			}
			for _, text := range tc.want {
				assert.Contains(t, body, text)
			}
			assert.NotContains(t, body, "private-token")
			assert.Nil(t, integration.lastCreds, "setup GET must not reconfigure the adapter")
		})
	}
}

func TestPantherDetailRedirect(t *testing.T) {
	ws, reg, _ := setupTestWeb()
	reg.integrations["panther"] = panther.New()
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/integrations/panther", nil))
	assert.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "/integrations/panther/setup", rr.Header().Get("Location"))
}

func postPantherForm(ws *WebServer, path string, form url.Values, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, req)
	return rr
}

func TestPantherInstanceSave(t *testing.T) {
	signedIn := mcp.Credentials{
		"instance_url":         "https://acme.runpanther.net",
		"api_token":            "",
		"mcp_access_token":     "oauth-token",
		"mcp_refresh_token":    "refresh",
		"mcp_client_id":        "client",
		mcp.CredKeyTokenSource: "oauth",
	}
	for _, tc := range []struct {
		name       string
		creds      mcp.Credentials
		form       url.Values
		header     map[string]string
		wantStatus int
		wantFlash  string
		want       mcp.Credentials
	}{
		{name: "first save normalizes", creds: mcp.Credentials{}, form: url.Values{"cred_instance_url": {" acme.runpanther.net/ "}}, wantStatus: http.StatusSeeOther, wantFlash: "Instance URL saved", want: mcp.Credentials{"instance_url": "https://acme.runpanther.net", "api_token": "", "mcp_access_token": "", "mcp_refresh_token": "", "mcp_token_expiry": "", "mcp_token_issued": "", "mcp_client_id": "", "mcp_client_secret": "", mcp.CredKeyTokenSource: ""}},
		{name: "same instance keeps the sign-in", creds: signedIn, form: url.Values{"cred_instance_url": {"https://acme.runpanther.net/"}}, wantStatus: http.StatusSeeOther, wantFlash: "Instance URL unchanged", want: signedIn},
		{name: "new instance clears both modes", creds: mcp.Credentials{"instance_url": "https://acme.runpanther.net", "api_token": "old-token"}, form: url.Values{"cred_instance_url": {"https://other.runpanther.net"}}, wantStatus: http.StatusSeeOther, wantFlash: "Instance URL saved",
			want: mcp.Credentials{"instance_url": "https://other.runpanther.net", "api_token": "", "mcp_access_token": "", "mcp_refresh_token": "", "mcp_token_expiry": "", "mcp_token_issued": "", "mcp_client_id": "", "mcp_client_secret": "", mcp.CredKeyTokenSource: ""}},
		{name: "insecure URL refused", creds: signedIn, form: url.Values{"cred_instance_url": {"http://acme.runpanther.net"}}, wantStatus: http.StatusSeeOther, wantFlash: "must use https", want: signedIn},
		{name: "cross-site post refused", creds: signedIn, form: url.Values{"cred_instance_url": {"https://evil.runpanther.net"}}, header: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantStatus: http.StatusForbidden, want: signedIn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			integration := panther.New()
			t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
			reg.integrations["panther"] = integration
			cfg.cfg.Integrations["panther"] = &mcp.IntegrationConfig{Credentials: maps.Clone(tc.creds)}
			rr := postPantherForm(ws, "/integrations/panther/instance", tc.form, tc.header)
			require.Equal(t, tc.wantStatus, rr.Code)
			if tc.wantFlash != "" {
				location, err := url.Parse(rr.Header().Get("Location"))
				require.NoError(t, err)
				assert.Contains(t, location.Query().Get("result")+location.Query().Get("error"), tc.wantFlash)
			}
			assert.Equal(t, tc.want, cfg.cfg.Integrations["panther"].Credentials)
		})
	}
}

// pantherAPIFixture fakes Panther's REST API: the token in X-API-Key, 401
// for any other token, and the token self-lookup used as the health check.
func pantherAPIFixture(t *testing.T, token string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "invalid API token"})
			return
		}
		switch r.URL.Path {
		case "/api-tokens/self":
			writeJSON(w, http.StatusOK, map[string]any{"id": "tok-fixture", "name": "switchboard", "permissions": []string{"AlertRead"}})
		case "/alerts":
			writeJSON(w, http.StatusOK, map[string]any{"results": []map[string]any{{"id": "alert-fixture", "title": "Fixture root login", "severity": "HIGH", "status": "OPEN"}}})
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "not found: " + r.URL.Path})
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestPantherAPITokenSave(t *testing.T) {
	api := pantherAPIFixture(t, "good-token")
	signedIn := mcp.Credentials{
		"instance_url":         api.URL,
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
		wantToken   string
		wantEnabled bool
		wantHealthy bool
		unchanged   bool
	}{
		{name: "instance required", creds: mcp.Credentials{}, form: url.Values{"enabled": {"true"}, "cred_api_token": {"good-token"}}, wantStatus: http.StatusSeeOther, wantFlash: "Save your Panther instance URL first", unchanged: true},
		{name: "token required", creds: mcp.Credentials{"instance_url": api.URL}, form: url.Values{"enabled": {"true"}}, wantStatus: http.StatusSeeOther, wantFlash: "API token is required", unchanged: true},
		{name: "switches a sign-in to the API token", creds: signedIn, form: url.Values{"enabled": {"true"}, "cred_api_token": {" good-token "}}, wantStatus: http.StatusSeeOther, wantFlash: "Configuration saved", wantToken: "good-token", wantEnabled: true, wantHealthy: true},
		{name: "blank token keeps the saved token", creds: mcp.Credentials{"instance_url": api.URL, "api_token": "good-token"}, form: url.Values{"cred_api_token": {""}}, wantStatus: http.StatusSeeOther, wantFlash: "Configuration saved", wantToken: "good-token"},
		{name: "rejected token saves but is unhealthy", creds: mcp.Credentials{"instance_url": api.URL}, form: url.Values{"enabled": {"true"}, "cred_api_token": {"bad-token"}}, wantStatus: http.StatusSeeOther, wantFlash: "Panther did not accept the API token", wantToken: "bad-token", wantEnabled: true},
		{name: "cross-site post refused", creds: mcp.Credentials{"instance_url": api.URL}, form: url.Values{"cred_api_token": {"good-token"}}, header: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantStatus: http.StatusForbidden, unchanged: true},
		{name: "instance and token save together", creds: mcp.Credentials{}, form: url.Values{"enabled": {"true"}, "cred_instance_url": {api.URL + "/"}, "cred_api_token": {"good-token"}}, wantStatus: http.StatusSeeOther, wantFlash: "Configuration saved", wantToken: "good-token", wantEnabled: true, wantHealthy: true},
		{name: "a new instance needs its own token", creds: mcp.Credentials{"instance_url": api.URL, "api_token": "good-token"}, form: url.Values{"enabled": {"true"}, "cred_instance_url": {"https://other.runpanther.net"}}, wantStatus: http.StatusSeeOther, wantFlash: "API token is required", unchanged: true},
		{name: "invalid instance refused", creds: mcp.Credentials{"instance_url": api.URL, "api_token": "good-token"}, form: url.Values{"cred_instance_url": {"http://acme.runpanther.net"}, "cred_api_token": {"good-token"}}, wantStatus: http.StatusSeeOther, wantFlash: "must use https", unchanged: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			integration := panther.New()
			t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
			reg.integrations["panther"] = integration
			before := maps.Clone(tc.creds)
			cfg.cfg.Integrations["panther"] = &mcp.IntegrationConfig{Credentials: maps.Clone(tc.creds)}
			rr := postPantherForm(ws, "/integrations/panther/api-token", tc.form, tc.header)
			require.Equal(t, tc.wantStatus, rr.Code)
			saved := cfg.cfg.Integrations["panther"]
			if tc.unchanged {
				assert.Equal(t, before, saved.Credentials)
			} else {
				assert.Equal(t, tc.wantToken, saved.Credentials["api_token"])
				assert.Equal(t, api.URL, saved.Credentials["instance_url"])
				assert.Equal(t, tc.wantEnabled, saved.Enabled)
				assert.Equal(t, "api_token", saved.Credentials[mcp.CredKeyTokenSource])
				for _, key := range pantherSignInKeys {
					assert.Empty(t, saved.Credentials[key], "%s must be cleared", key)
				}
			}
			if tc.wantFlash != "" {
				location, err := url.Parse(rr.Header().Get("Location"))
				require.NoError(t, err)
				assert.Contains(t, location.Query().Get("result")+location.Query().Get("error"), tc.wantFlash)
			}
			if tc.wantEnabled {
				entry, ok := ws.health.get("panther")
				require.True(t, ok)
				assert.Equal(t, tc.wantHealthy, entry.Healthy)
			}
		})
	}
}

// pantherOAuthFixture fakes Panther Remote MCP sign-in on one origin, like
// api.<instance>: protected resource metadata for /mcp naming the origin as
// issuer, authorization server metadata without scopes_supported (Panther
// documents no scope strings), dynamic client registration as a public
// client, and the MCP server at /mcp.
type pantherOAuthFixture struct {
	server     *httptest.Server
	mu         sync.Mutex
	tokenForm  url.Values
	registered map[string]any
}

func newPantherOAuthFixture(t *testing.T) *pantherOAuthFixture {
	t.Helper()
	f := &pantherOAuthFixture{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "panther-fixture", Version: "1"}, nil)
	server.AddTool(&mcpsdk.Tool{Name: "list_alerts", Description: "List Panther alerts.", InputSchema: map[string]any{"type": "object"}, Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true}},
		func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "{}"}}, StructuredContent: map[string]any{"alerts": []any{map[string]any{"id": "remote-alert", "severity": "HIGH"}}}}, nil
		})
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"resource": f.server.URL + "/mcp", "authorization_servers": []string{f.server.URL}})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/oauth/authorize",
			"token_endpoint": f.server.URL + "/oauth/token", "registration_endpoint": f.server.URL + "/oauth/register",
			"code_challenge_methods_supported": []string{"S256"},
		})
	})
	mux.HandleFunc("POST /oauth/register", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.registered = body
		f.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]string{"client_id": "panther-client"})
	})
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.tokenForm = r.PostForm
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "panther-oauth-token", "token_type": "Bearer", "refresh_token": "panther-refresh-token", "expires_in": 3600})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer panther-oauth-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func startPantherOAuth(t *testing.T, ws *WebServer, header map[string]string) map[string]string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/remote/panther/oauth/start", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	ws.Handler().ServeHTTP(rr, req)
	var started map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &started), rr.Body.String())
	return started
}

func TestPantherOAuthFlow(t *testing.T) {
	f := newPantherOAuthFixture(t)
	ws, reg, _ := setupTestWeb()
	t.Setenv("HOME", t.TempDir())
	store, err := config.NewManager()
	require.NoError(t, err)
	ws.services.Config = store
	initial := mcp.Credentials{"instance_url": f.server.URL, "api_token": "previous-api-token"}
	require.NoError(t, store.SetIntegration("panther", &mcp.IntegrationConfig{Credentials: initial}))
	integration := panther.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	reg.integrations["panther"] = integration

	started := startPantherOAuth(t, ws, nil)
	require.Empty(t, started["error"])
	authorizeURL, err := url.Parse(started["authorize_url"])
	require.NoError(t, err)
	auth := authorizeURL.Query()
	assert.Equal(t, f.server.URL+"/oauth/authorize", authorizeURL.Scheme+"://"+authorizeURL.Host+authorizeURL.Path)
	assert.False(t, auth.Has("scope"), "no advertised scopes means no scope parameter")
	assert.False(t, auth.Has("prompt"))
	assert.Equal(t, f.server.URL+"/mcp", auth.Get("resource"))
	assert.Equal(t, "S256", auth.Get("code_challenge_method"))
	assert.Equal(t, "http://localhost:3847/api/remote/panther/oauth/callback", auth.Get("redirect_uri"), "a loopback redirect, which Panther always allows")
	assert.Equal(t, "panther-client", auth.Get("client_id"))
	f.mu.Lock()
	assert.Equal(t, []any{"http://localhost:3847/api/remote/panther/oauth/callback"}, f.registered["redirect_uris"])
	assert.Equal(t, "none", f.registered["token_endpoint_auth_method"], "Panther treats MCP clients as public clients")
	f.mu.Unlock()

	rr := httptest.NewRecorder()
	query := url.Values{"code": {"panther-code"}, "state": {auth.Get("state")}}
	ws.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/remote/panther/oauth/callback?"+query.Encode(), nil))
	require.Equal(t, http.StatusSeeOther, rr.Code)
	location, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/integrations/panther/setup", location.Path)
	assert.Equal(t, "Connected via MCP OAuth", location.Query().Get("result"))

	require.NoError(t, store.Load())
	saved, _ := store.GetIntegration("panther")
	assert.True(t, saved.Enabled)
	assert.Equal(t, "panther-oauth-token", saved.Credentials["mcp_access_token"])
	assert.Empty(t, saved.Credentials["api_token"], "signing in replaces API token mode")
	assert.Equal(t, f.server.URL, saved.Credentials["instance_url"])
	assert.Equal(t, "panther-refresh-token", saved.Credentials["mcp_refresh_token"])
	assert.Equal(t, "panther-client", saved.Credentials["mcp_client_id"])
	expiry, err := time.Parse(time.RFC3339, saved.Credentials["mcp_token_expiry"])
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(time.Hour), expiry, time.Minute)

	assert.True(t, integration.Healthy(t.Context()), "adapter must use the new login immediately")
	tools := integration.Tools()
	require.Len(t, tools, 1)
	assert.Equal(t, mcp.ToolName("panther_list_alerts"), tools[0].Name)
	assert.Equal(t, mcp.AccessRead, tools[0].Access)
	result, err := integration.Execute(t.Context(), "panther_list_alerts", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.JSONEq(t, `{"alerts":[{"id":"remote-alert","severity":"HIGH"}]}`, result.Data)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "panther-code", f.tokenForm.Get("code"))
	assert.Equal(t, auth.Get("resource"), f.tokenForm.Get("resource"))
	assert.NotEmpty(t, f.tokenForm.Get("code_verifier"))
}

func TestPantherOAuthStartErrors(t *testing.T) {
	broken := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(broken.Close)
	for _, tc := range []struct {
		name   string
		creds  mcp.Credentials
		header map[string]string
		want   string
	}{
		{name: "no instance saved", creds: mcp.Credentials{}, want: "Save your Panther instance URL first"},
		{name: "instance without Remote MCP", creds: mcp.Credentials{"instance_url": broken.URL}, want: "use an API token"},
		{name: "cross-site start refused", creds: mcp.Credentials{"instance_url": broken.URL}, header: map[string]string{"Sec-Fetch-Site": "cross-site"}, want: "Same-origin request required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, reg, cfg := setupTestWeb()
			reg.integrations["panther"] = &mockIntegration{name: "panther"}
			cfg.cfg.Integrations["panther"] = &mcp.IntegrationConfig{Credentials: tc.creds}
			started := startPantherOAuth(t, ws, tc.header)
			assert.Contains(t, started["error"], tc.want)
			assert.Empty(t, started["authorize_url"])
		})
	}
}
