package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const (
	uiRenderTools = 48
	uiRenderKey   = "rnd_fixture"
)

// uiRenderFixture fakes Render's hosted MCP server: it only speaks the legacy
// session handshake and, like mcp.render.com, accepts any bearer token at the
// transport and reports a bad API key as a tool error from the Render API.
type uiRenderFixture struct {
	server     *httptest.Server
	mu         sync.Mutex
	workspaces []string
}

func newUIRenderFixture(t *testing.T) *uiRenderFixture {
	t.Helper()
	fixture := &uiRenderFixture{}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-render", Version: "1"}, nil)
	replies := map[string]string{
		"list_workspaces": `[{"id":"tea-fixture","name":"Fixture Team","email":"ops@example.com","type":"team"}]`,
		"list_services":   `[{"id":"srv-fixture","name":"Fixture API","type":"web_service","suspended":"not_suspended","autoDeploy":"yes","dashboardUrl":"https://dashboard.render.com/web/srv-fixture","notifyOnFail":"SECRET_SENTINEL","serviceDetails":{"url":"https://fixture.onrender.com","region":"oregon","plan":"starter","runtime":"node","openPorts":[{"port":10000,"protocol":"SECRET_SENTINEL"}]}}]`,
	}
	for name, reply := range replies {
		server.AddTool(&mcpsdk.Tool{Name: name, InputSchema: map[string]any{"type": "object"}}, func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			var args map[string]any
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			if req.Extra.Header.Get("Authorization") != "Bearer "+uiRenderKey {
				return &mcpsdk.CallToolResult{IsError: true, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "received response code 401: unauthorized"}}}, nil
			}
			if workspace, ok := args["workspaceId"].(string); ok {
				fixture.mu.Lock()
				fixture.workspaces = append(fixture.workspaces, workspace)
				fixture.mu.Unlock()
			}
			return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: reply}}}, nil
		})
	}
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server }, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var message struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			_ = json.Unmarshal(body, &message)
			if message.Method == "server/discover" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32601,"message":"Method not found"}}`, message.ID)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *uiRenderFixture) lastWorkspace() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.workspaces) == 0 {
		return ""
	}
	return f.workspaces[len(f.workspaces)-1]
}

// uiRenderMCP searches for Render tools over MCP and, when they are enabled,
// lists services through execute.
func uiRenderMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "render-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "render", "limit": 100}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "render_list_services")
	require.NotContains(t, names, "render_select_workspace")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "render_list_services", "arguments": map[string]any{}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "Fixture API")
	require.Contains(t, text.Text, "https://fixture.onrender.com")
	require.NotContains(t, text.Text, "SECRET_SENTINEL", "compaction must drop notification settings and ports")
	return names
}

func TestUIRender(t *testing.T) {
	app := newUIApp(t)
	fixture := newUIRenderFixture(t)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/render"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiRenderMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct {
		key, kind, hint string
		optional        bool
	}{
		{"api_key", "password", "Account Settings > API Keys", false},
		{"workspace_id", "text", "tea-", true},
		{"base_url", "text", "https://mcp.render.com", true},
	} {
		input := form.Locator("#cred_" + field.key)
		kind, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.kind, kind)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, placeholder, field.hint)
		label, err := form.Locator(`label[for="cred_` + field.key + `"]`).TextContent()
		require.NoError(t, err)
		want := field.key
		if field.optional {
			want += " (optional)"
		}
		require.Equal(t, want, label)
	}
	uiText(t, page.Locator("body"), fmt.Sprintf("Available Tools (%d)", uiRenderTools))

	require.NoError(t, form.Locator("#cred_base_url").Fill(fixture.server.URL))
	require.NoError(t, form.Locator("#cred_api_key").Fill("rnd_revoked"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")

	require.NoError(t, form.Locator("#cred_api_key").Fill(uiRenderKey))
	require.NoError(t, form.Locator("#cred_workspace_id").Fill("tea-fixture"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiRenderMCP(t, app), uiRenderTools)
	require.Equal(t, "tea-fixture", fixture.lastWorkspace(), "the saved workspace is the default for scoped tools")
	uiCapture(t, page, "render-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "render-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/render")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiRenderMCP(t, app), uiRenderTools)

	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiRenderMCP(t, app))
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["render"].Enabled)
	require.Equal(t, fixture.server.URL, saved.Integrations["render"].Credentials["base_url"])
	require.Equal(t, "tea-fixture", saved.Integrations["render"].Credentials["workspace_id"])
}
