package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// uiMobbinMCP searches for Mobbin tools on the running binary and, when they
// are discoverable, lists collections from the fixture.
func uiMobbinMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "mobbin", "integration": "mobbin", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "mobbin_search_screens")

	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{
		"tool_name": "mobbin_list_collections",
		"arguments": map[string]any{},
	}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "collections")
	return names
}

// uiMobbinAuthorize answers the fixture's authorize URL in the browser the way
// Mobbin's sign-in page would, by redirecting to the Switchboard callback.
func uiMobbinAuthorize(t *testing.T, page playwright.Page, fixture *mobbinOAuthFixture, app *uiApp, cancel bool) {
	t.Helper()
	require.NoError(t, page.Route(fixture.server.URL+"/auth/v1/oauth/authorize*", func(route playwright.Route) {
		authorize, err := url.Parse(route.Request().URL())
		require.NoError(t, err)
		callback, err := url.Parse(app.URL + "/api/remote/mobbin/oauth/callback")
		require.NoError(t, err)
		query := callback.Query()
		query.Set("state", authorize.Query().Get("state"))
		if cancel {
			query.Set("error", "access_denied")
		} else {
			query.Set("code", "fixture-code")
		}
		callback.RawQuery = query.Encode()
		require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(http.StatusFound), Headers: map[string]string{"Location": callback.String()}}))
	}))
}

func uiMobbinSaved(t *testing.T, app *uiApp) *mcp.IntegrationConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	return saved.Integrations["mobbin"]
}

func TestUIMobbin(t *testing.T) {
	app := newUIApp(t)
	fixture := newMobbinOAuthFixture(t)

	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/mobbin"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiMobbinMCP(t, app), "a fresh install must not expose Mobbin tools")
	require.NoError(t, card.Click())
	uiText(t, page.Locator("h1"), "Mobbin Setup")
	uiText(t, page.Locator(".badge").First(), "Not Connected")
	uiText(t, page.Locator("#mobbin-oauth-start"), "Sign in with Mobbin")
	require.Equal(t, "/integrations/mobbin/setup", mustURLPath(t, page.URL()))
	saved := uiMobbinSaved(t, app)
	require.NotNil(t, saved, "fresh config must list mobbin")
	require.Empty(t, saved.Credentials["mcp_access_token"])

	// Mobbin access tokens last an hour and are not refreshed, so the common
	// path is reauthorizing an expired token against the configured server.
	saved.Enabled = true
	saved.Credentials["base_url"] = fixture.server.URL
	saved.Credentials["mcp_access_token"] = "expired-token"
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))
	config["integrations"].(map[string]any)["mobbin"] = saved
	data, err = json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(app.home, ".config", "switchboard", "config.json"), data, 0o600))
	app.restart()

	t.Run("cancel", func(t *testing.T) {
		page := uiPage(t)
		uiMobbinAuthorize(t, page, fixture, app, true)
		uiNavigate(t, page, app.URL+"/integrations/mobbin")
		require.NoError(t, page.Locator("#mobbin-oauth-start").Click())
		uiText(t, page.Locator(".flash-error").First(), "access_denied")
		require.Equal(t, "expired-token", uiMobbinSaved(t, app).Credentials["mcp_access_token"])
	})

	uiMobbinAuthorize(t, page, fixture, app, false)
	uiNavigate(t, page, app.URL+"/integrations/mobbin")
	uiText(t, page.Locator("#mobbin-oauth-start"), "Re-authorize with Mobbin")
	require.NoError(t, page.Locator("#mobbin-oauth-start").Click())
	uiText(t, page.Locator(".flash-success"), "Connected via MCP OAuth")
	content, err := page.Content()
	require.NoError(t, err)
	require.NotContains(t, content, "mobbin-oauth-token")
	saved = uiMobbinSaved(t, app)
	require.True(t, saved.Enabled)
	require.Equal(t, "mobbin-oauth-token", saved.Credentials["mcp_access_token"])
	require.Equal(t, "oauth", saved.Credentials[mcp.CredKeyTokenSource])
	require.Equal(t, fixture.server.URL, saved.Credentials["base_url"])
	require.Len(t, uiMobbinMCP(t, app), 8)
	uiCapture(t, page, "mobbin-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiCapture(t, page, "mobbin-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/mobbin")
	require.Eventually(t, func() bool {
		if _, err := page.Reload(); err != nil {
			return false
		}
		text, err := page.Locator(".badge").First().TextContent()
		return err == nil && text == "Connected"
	}, 20*time.Second, 500*time.Millisecond, "restarted server must report Mobbin connected")
	require.Len(t, uiMobbinMCP(t, app), 8)
}

func mustURLPath(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	return parsed.Path
}
