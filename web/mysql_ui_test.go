package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/integrations/mysql"
	"github.com/daltoniam/switchboard/internal/mysqltest"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

func TestUIMySQL(t *testing.T) {
	app := newUIApp(t)
	fixture := mysqltest.Start(t)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/mysql"]`)
	uiText(t, card, "Disabled")
	require.NoError(t, card.Click())
	uiText(t, page.Locator("h1"), "mysql")
	form := page.Locator("#integration-configuration")
	for _, key := range []string{"connection_string", "connections", "host", "port", "user", "password", "database", "tls"} {
		input := form.Locator("#cred_" + key)
		kind, err := input.GetAttribute("type")
		require.NoError(t, err)
		expected := "text"
		if key == "connection_string" || key == "connections" || key == "password" {
			expected = "password"
		}
		require.Equal(t, expected, kind)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.NotEmpty(t, placeholder)
		uiText(t, form.Locator(`label[for="cred_`+key+`"]`), "optional")
	}
	uiText(t, page.Locator("body"), "mysql_query")
	uiText(t, page.Locator("body"), "not a SQL sandbox")
	require.NoError(t, form.Locator("#cred_host").Fill("127.0.0.1"))
	require.NoError(t, form.Locator("#cred_port").Fill(fixture.Port))
	require.NoError(t, form.Locator("#cred_user").Fill("reader"))
	require.NoError(t, form.Locator("#cred_password").Fill(mysqltest.Password))
	require.NoError(t, form.Locator("#cred_database").Fill("app"))
	require.NoError(t, form.Locator("#cred_tls").Fill("false"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	mysqlUISearch(t, app, true)
	uiCapture(t, page, "mysql-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Close sidebar", Exact: playwright.Bool(true)}).Click())
	uiCapture(t, page, "mysql-mobile")
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/mysql")
	uiText(t, page.Locator(".badge").First(), "Connected")
	mysqlUISearch(t, app, true)
	require.NoError(t, form.Locator("#cred_password").Fill("invalid-fixture"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill(mysqltest.Password))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	uiNavigate(t, page, app.URL+"/integrations")
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all"}).Click())
	uiText(t, card, "Connected")
	require.NoError(t, card.Click())
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	mysqlUISearch(t, app, false)
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/mysql")
	uiText(t, page.Locator(".badge").First(), "Disabled")
	t.Logf("Isolated binary URL: %s; MySQL wire fixture, not live authentication", app.URL)
}

func mysqlUISearch(t *testing.T, app *uiApp, enabled bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mysql-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer session.Close()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "mysql", "query": "accessible schemas discovery", "limit": 1}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text := result.Content[0].(*mcpsdk.TextContent).Text
	var response struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &response))
	if !enabled {
		require.Empty(t, response.Tools)
		return
	}
	var names []string
	for _, tool := range response.Tools {
		names = append(names, tool.Name)
	}
	require.Contains(t, names, "mysql_list_databases")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "mysql_list_databases", "arguments": map[string]any{}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "app")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "mysql_query", "arguments": map[string]any{"query": "SELECT name FROM users"}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].(*mcpsdk.TextContent).Text, "MySQL UI fixture")
}

func TestMySQLFreshSetup(t *testing.T) {
	ws, registry, store := setupTestWeb()
	integration := mysql.New()
	t.Cleanup(func() { _ = integration.(io.Closer).Close() })
	registry.integrations["mysql"] = integration
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defaults, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, defaults.Load())
	configured, ok := defaults.GetIntegration("mysql")
	require.True(t, ok)
	require.NoError(t, store.SetIntegration("mysql", configured))
	request := httptest.NewRequest("GET", "/integrations/mysql", nil)
	response := httptest.NewRecorder()
	ws.Handler().ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	for _, key := range []string{"connection_string", "connections", "host", "port", "user", "password", "database", "tls"} {
		require.Contains(t, response.Body.String(), `id="cred_`+key+`"`)
	}
}
