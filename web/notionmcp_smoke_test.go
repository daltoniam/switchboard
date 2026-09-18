package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/integrations/notionmcp"
	"github.com/daltoniam/switchboard/registry"
	"github.com/daltoniam/switchboard/server"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotionMCPOAuthSearchExecuteSmoke(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := newNotionMCPOAuthFixture(t, http.StatusOK)
	store, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, store.SetIntegration("notion", &mcp.IntegrationConfig{
		Enabled: true, Credentials: mcp.Credentials{"token_v2": "legacy-token", "space_id": "legacy-space"}, ToolGlobs: []string{"notion_*"},
	}))
	initial := &mcp.IntegrationConfig{
		Enabled: false, Credentials: mcp.Credentials{"base_url": f.server.URL, "mcp_access_token": "previous-token"}, ToolGlobs: []string{"notion-mcp_*"},
	}
	require.NoError(t, store.SetIntegration("notion-mcp", initial))
	legacyBefore := notionConfigBytes(t)
	integration := notionmcp.New()
	t.Cleanup(func() { assert.NoError(t, integration.(io.Closer).Close()) })
	require.NoError(t, integration.Configure(t.Context(), initial.Credentials))
	reg := registry.New()
	require.NoError(t, reg.Register(integration))
	services := &mcp.Services{Config: store, Registry: reg}
	srv := server.New(services)
	hooks := 0
	ws := New(services, 3847, nil, nil, WithConfigChangeHook(func() {
		hooks++
		srv.RefreshSearchIndex()
	}))
	downstream := httptest.NewServer(srv.StatelessHandler())
	t.Cleanup(downstream.Close)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "notion-smoke", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint: downstream.URL, HTTPClient: downstream.Client(), DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, session.Close()) })

	call := func(name string, args map[string]any) string {
		t.Helper()
		result, callErr := session.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
		require.NoError(t, callErr)
		require.NotEmpty(t, result.Content)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.False(t, result.IsError, "%s", text.Text)
		return text.Text
	}
	searchArgs := map[string]any{"query": "search", "integration": "notion-mcp"}
	var before struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(call("search", searchArgs)), &before))
	require.Zero(t, before.Total)
	require.Zero(t, hooks)

	auth := startNotionMCPOAuth(t, ws)
	location := notionMCPCallback(t, ws, url.Values{"code": {"notion-code"}, "state": {auth.Get("state")}})
	require.Empty(t, location.Query().Get("error"))
	require.Equal(t, "Connected via MCP OAuth", location.Query().Get("result"))
	require.Equal(t, 1, hooks)
	var after struct {
		Total int `json:"total"`
		Tools []struct {
			Name        string `json:"name"`
			Integration string `json:"integration"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(call("search", searchArgs)), &after))
	require.Equal(t, 1, after.Total, "the callback must refresh scored search on the existing server")
	require.Len(t, after.Tools, 1)
	assert.Equal(t, "notion-mcp_notion-search", after.Tools[0].Name)
	assert.Equal(t, "notion-mcp", after.Tools[0].Integration)
	result := call("execute", map[string]any{
		"tool_name": after.Tools[0].Name, "arguments": map[string]any{"query": "smoke page"},
	})
	assert.JSONEq(t, `{"id":"fixture-page"}`, result)
	f.mu.Lock()
	toolCalls, toolArgs := f.toolCalls, f.toolArgs
	f.mu.Unlock()
	assert.Equal(t, 1, toolCalls)
	assert.JSONEq(t, `{"query":"smoke page"}`, string(toolArgs))
	durable, err := config.NewManager()
	require.NoError(t, err)
	saved, ok := durable.GetIntegration("notion-mcp")
	require.True(t, ok)
	assert.True(t, saved.Enabled)
	assert.Equal(t, "notion-oauth-token", saved.Credentials["mcp_access_token"])
	assert.Equal(t, legacyBefore, notionConfigBytes(t), "the original Notion configuration must remain byte-unchanged")
}

func notionConfigBytes(t *testing.T) json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved struct {
		Integrations map[string]json.RawMessage `json:"integrations"`
	}
	require.NoError(t, json.Unmarshal(data, &saved))
	require.NotEmpty(t, saved.Integrations["notion"])
	return saved.Integrations["notion"]
}
