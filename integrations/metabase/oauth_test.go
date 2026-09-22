package metabase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hostedMCPFixture struct {
	server     *httptest.Server
	mu         sync.Mutex
	validToken string
	toolCalls  int
	refreshes  int
	properties map[string]any
	propStatus int
}

func newHostedMCPFixture(t *testing.T) *hostedMCPFixture {
	t.Helper()
	f := &hostedMCPFixture{validToken: "access-1", properties: map[string]any{"mcp-enabled?": true, "site-name": "Fixture"}, propStatus: http.StatusOK}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "metabase-fixture", Version: "1"}, nil)
	for _, name := range []string{"search", "execute_query"} {
		server.AddTool(&mcpsdk.Tool{Name: name, Description: "Hosted " + name, InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			f.mu.Lock()
			f.toolCalls++
			f.mu.Unlock()
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: `{"hosted":true}`}}}, nil
		})
	}
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		writeFixtureJSON(w, http.StatusOK, map[string]any{"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/oauth/authorize", "token_endpoint": f.server.URL + "/oauth/token"})
	})
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "refresh_token", r.PostForm.Get("grant_type"))
		f.mu.Lock()
		f.refreshes++
		f.validToken = "access-2"
		f.mu.Unlock()
		writeFixtureJSON(w, http.StatusOK, map[string]string{"access_token": "access-2", "refresh_token": "refresh-2", "token_type": "Bearer"})
	})
	mux.HandleFunc("GET /api/session/properties", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		writeFixtureJSON(w, f.propStatus, f.properties)
	})
	mux.HandleFunc("GET /api/user/current", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "mb_key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeFixtureJSON(w, http.StatusOK, map[string]any{"id": 1})
	})
	mux.HandleFunc("/api/metabase-mcp", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		valid := f.validToken
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func writeFixtureJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type fixtureConfig struct {
	mu   sync.Mutex
	ic   *mcp.IntegrationConfig
	sets int
}

func (c *fixtureConfig) Load() error                                 { return nil }
func (c *fixtureConfig) Save() error                                 { return nil }
func (c *fixtureConfig) Get() *mcp.Config                            { return nil }
func (c *fixtureConfig) Update(*mcp.Config) error                    { return nil }
func (c *fixtureConfig) SetWasmModules([]mcp.WasmModuleConfig) error { return nil }
func (c *fixtureConfig) EnabledIntegrations() []string               { return nil }
func (c *fixtureConfig) DefaultCredentialKeys(string) []string       { return nil }
func (c *fixtureConfig) GetIntegration(string) (*mcp.IntegrationConfig, bool) {
	return c.ic, c.ic != nil
}
func (c *fixtureConfig) SetIntegration(_ string, ic *mcp.IntegrationConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ic = ic
	c.sets++
	return nil
}

func TestConfigure_Modes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		creds      mcp.Credentials
		wantErr    string
		wantRemote bool
	}{
		{name: "url required", creds: mcp.Credentials{"api_key": "k"}, wantErr: "url is required"},
		{name: "one credential required", creds: mcp.Credentials{"url": "https://mb.example.com"}, wantErr: "api_key or mcp_access_token is required"},
		{name: "api key only", creds: mcp.Credentials{"url": "https://mb.example.com", "api_key": "k"}},
		{name: "oauth token only", creds: mcp.Credentials{"url": "https://mb.example.com", "mcp_access_token": "t"}, wantRemote: true},
		{name: "both prefers oauth", creds: mcp.Credentials{"url": "https://mb.example.com", "api_key": "k", "mcp_access_token": "t"}, wantRemote: true},
		{name: "both with api_key source", creds: mcp.Credentials{"url": "https://mb.example.com", "api_key": "k", "mcp_access_token": "t", mcp.CredKeyTokenSource: "api_key"}},
		{name: "both with oauth source", creds: mcp.Credentials{"url": "https://mb.example.com", "api_key": "k", "mcp_access_token": "t", mcp.CredKeyTokenSource: "oauth"}, wantRemote: true},
		{name: "api_key source without key falls back to oauth", creds: mcp.Credentials{"url": "https://mb.example.com/", "mcp_access_token": "t", mcp.CredKeyTokenSource: "api_key"}, wantRemote: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := New()
			t.Cleanup(func() { _ = i.(io.Closer).Close() })
			err := i.Configure(t.Context(), tc.creds)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRemote, IsRemoteMCP(i))
			assert.Equal(t, "https://mb.example.com", MCPServerURL(i))
		})
	}
}

func TestMCPServerURL_Unconfigured(t *testing.T) {
	assert.Empty(t, MCPServerURL(New()))
	assert.Empty(t, MCPServerURL(&fixtureIntegration{}))
	assert.False(t, IsRemoteMCP(&fixtureIntegration{}))
}

type fixtureIntegration struct{ mcp.Integration }

func TestRemoteMode_ProxiesHostedMCP(t *testing.T) {
	f := newHostedMCPFixture(t)
	i := New()
	t.Cleanup(func() { _ = i.(io.Closer).Close() })
	require.NoError(t, i.Configure(t.Context(), mcp.Credentials{"url": f.server.URL, "mcp_access_token": "access-1"}))

	assert.True(t, i.Healthy(t.Context()))
	tools := i.Tools()
	require.Len(t, tools, 2)
	byName := map[mcp.ToolName]mcp.ToolDefinition{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	require.Contains(t, byName, mcp.ToolName("metabase_search"))
	assert.Contains(t, byName["metabase_search"].Description, "Start here")
	assert.NotContains(t, byName["metabase_execute_query"].Description, "Start here")

	result, err := i.Execute(t.Context(), "metabase_execute_query", map[string]any{"query": map[string]any{}})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, "hosted")

	result, err = i.Execute(t.Context(), "metabase_list_databases", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError, "REST tools are not served in OAuth mode")

	fc := i.(mcp.FieldCompactionIntegration)
	_, ok := fc.CompactSpec("metabase_search")
	assert.False(t, ok, "REST compaction specs must not apply to hosted MCP results")
	_, ok = i.(mcp.ToolMaxBytesIntegration).MaxBytes("metabase_search")
	assert.False(t, ok)
}

func TestAPIKeyMode_KeepsRESTBehavior(t *testing.T) {
	f := newHostedMCPFixture(t)
	i := New()
	require.NoError(t, i.Configure(t.Context(), mcp.Credentials{"url": f.server.URL, "api_key": "mb_key", "mcp_access_token": "access-1", mcp.CredKeyTokenSource: "api_key"}))
	assert.False(t, IsRemoteMCP(i))
	assert.True(t, i.Healthy(t.Context()))
	assert.Equal(t, tools, i.Tools())
	_, ok := i.(mcp.FieldCompactionIntegration).CompactSpec("metabase_search")
	assert.True(t, ok)
}

func TestRemoteMode_RefreshPersistsThroughConfigService(t *testing.T) {
	f := newHostedMCPFixture(t)
	i := New()
	t.Cleanup(func() { _ = i.(io.Closer).Close() })
	cfg := &fixtureConfig{ic: &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{
		"url": f.server.URL, "api_key": "mb_key", "mcp_access_token": "stale", "mcp_refresh_token": "refresh-1", "mcp_client_id": "client-1", mcp.CredKeyTokenSource: "oauth",
	}}}
	SetConfigService(i, cfg)
	require.NoError(t, i.Configure(t.Context(), cfg.ic.Credentials))

	result, err := i.Execute(t.Context(), "metabase_search", map[string]any{"q": "revenue"})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)

	f.mu.Lock()
	assert.Equal(t, 1, f.refreshes)
	assert.Equal(t, 1, f.toolCalls)
	f.mu.Unlock()
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	assert.Equal(t, 1, cfg.sets)
	assert.Equal(t, "access-2", cfg.ic.Credentials["mcp_access_token"])
	assert.Equal(t, "refresh-2", cfg.ic.Credentials["mcp_refresh_token"])
	assert.Equal(t, "client-1", cfg.ic.Credentials["mcp_client_id"])
	assert.Equal(t, "mb_key", cfg.ic.Credentials["api_key"], "the API-key fallback must survive a refresh")
	assert.True(t, cfg.ic.Enabled)
}

func TestSetConfigService_IgnoresOtherIntegrations(t *testing.T) {
	SetConfigService(&fixtureIntegration{}, &fixtureConfig{})
}

func TestConfigure_SwitchingURLReplacesRemote(t *testing.T) {
	first := newHostedMCPFixture(t)
	second := newHostedMCPFixture(t)
	i := New()
	t.Cleanup(func() { _ = i.(io.Closer).Close() })
	require.NoError(t, i.Configure(t.Context(), mcp.Credentials{"url": first.server.URL, "mcp_access_token": "access-1"}))
	require.True(t, i.Healthy(t.Context()))
	require.NoError(t, i.Configure(t.Context(), mcp.Credentials{"url": second.server.URL, "mcp_access_token": "access-1"}))
	require.True(t, i.Healthy(t.Context()))
	first.mu.Lock()
	second.mu.Lock()
	defer first.mu.Unlock()
	defer second.mu.Unlock()
	assert.Equal(t, second.server.URL, MCPServerURL(i))
}

func TestClose_ThenExecuteReportsNotConfigured(t *testing.T) {
	f := newHostedMCPFixture(t)
	i := New()
	require.NoError(t, i.Configure(t.Context(), mcp.Credentials{"url": f.server.URL, "mcp_access_token": "access-1"}))
	require.NoError(t, i.(io.Closer).Close())
	require.NoError(t, i.(io.Closer).Close())
	result, err := i.Execute(t.Context(), "metabase_search", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestMCPServerEnabled(t *testing.T) {
	for _, tc := range []struct {
		name       string
		properties map[string]any
		status     int
		wantOn     bool
		wantErr    string
	}{
		{name: "enabled", properties: map[string]any{"mcp-enabled?": true}, status: http.StatusOK, wantOn: true},
		{name: "disabled", properties: map[string]any{"mcp-enabled?": false}, status: http.StatusOK},
		{name: "setting missing", properties: map[string]any{"site-name": "x"}, status: http.StatusOK, wantErr: "does not report"},
		{name: "http error", properties: map[string]any{}, status: http.StatusBadGateway, wantErr: "502"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHostedMCPFixture(t)
			f.mu.Lock()
			f.properties, f.propStatus = tc.properties, tc.status
			f.mu.Unlock()
			on, err := MCPServerEnabled(t.Context(), f.server.Client(), f.server.URL+"/")
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantOn, on)
		})
	}
}

func TestMCPServerEnabled_Unreachable(t *testing.T) {
	_, err := MCPServerEnabled(t.Context(), &http.Client{}, "http://127.0.0.1:1")
	assert.Error(t, err)
}
