package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestUIAshby drives the development binary through Ashby's hosted MCP
// OAuth setup against a local fixture; no real Ashby account is contacted.
func TestUIAshby(t *testing.T) {
	app := newUIApp(t)
	fixture := newAshbyOAuthFixture(t)
	page := uiPage(t)
	// Playwright runs route callbacks off the test goroutine, so problems
	// are recorded and checked here, and every request is answered.
	var routeMu sync.Mutex
	var routeProblems []string
	denied := false
	require.NoError(t, page.Route(fixture.server.URL+"/oidc/auth*", func(route playwright.Route) {
		routeMu.Lock()
		defer routeMu.Unlock()
		authorize, err := url.Parse(route.Request().URL())
		if err != nil {
			routeProblems = append(routeProblems, err.Error())
			_ = route.Abort()
			return
		}
		if got := authorize.Query().Get("prompt"); got != "consent" {
			routeProblems = append(routeProblems, "prompt="+got)
		}
		if got := authorize.Query().Get("resource"); got != fixture.server.URL+"/mcp/v1" {
			routeProblems = append(routeProblems, "resource="+got)
		}
		query := url.Values{"state": {authorize.Query().Get("state")}}
		if denied {
			query.Set("error", "access_denied")
		} else {
			query.Set("code", "fixture-code")
		}
		location := app.URL + "/api/remote/ashby/oauth/callback?" + query.Encode()
		if err := route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(http.StatusFound), Headers: map[string]string{"Location": location}}); err != nil {
			routeProblems = append(routeProblems, err.Error())
		}
	}))
	setDenied := func(value bool) {
		routeMu.Lock()
		defer routeMu.Unlock()
		denied = value
	}
	t.Cleanup(func() {
		routeMu.Lock()
		defer routeMu.Unlock()
		require.Empty(t, routeProblems, "authorize requests must carry prompt=consent and the /mcp/v1 resource")
	})

	// Fresh install: the card routes to the OAuth setup page, which asks
	// for no API key and renders no credential inputs.
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/ashby"]`)
	uiText(t, card, "ashby")
	uiText(t, card, "16 tools")
	uiText(t, card, "Disabled")
	require.NoError(t, card.Click())
	require.NoError(t, page.WaitForURL(app.URL+"/integrations/ashby/setup"))
	uiText(t, page.Locator("h1"), "Ashby Setup")
	uiText(t, page.Locator(".badge").First(), "Not Connected")
	uiText(t, page.Locator("body"), "Opt-In Features")
	uiText(t, page.Locator("#ashby-oauth-start"), "Sign in with Ashby")
	inputs, err := page.Locator("main input").Count()
	require.NoError(t, err)
	require.Zero(t, inputs)
	require.Empty(t, uiAshbySearch(t, app))
	uiCapture(t, page, "ashby-fresh-desktop")

	// A saved login the server rejects shows a visible connection error.
	app.stop()
	uiAshbySeed(t, app, mcp.Credentials{"base_url": fixture.server.URL, "mcp_access_token": "stale-fixture-token"})
	app.start()
	uiAshbyRecheck(t, page, app, "Not Verified")
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiText(t, page.Locator(".badge").First(), "Connection Error")
	uiText(t, page.Locator("#ashby-oauth-start"), "Re-authorize with Ashby")

	// Declining consent keeps the old login and shows the provider error.
	setDenied(true)
	require.NoError(t, page.Locator("#ashby-oauth-start").Click())
	uiText(t, page.Locator(".flash-error").First(), "access_denied")
	require.Equal(t, "stale-fixture-token", uiAshbySaved(t, app).Credentials["mcp_access_token"])

	setDenied(false)
	require.NoError(t, page.Locator("#ashby-oauth-start").Click())
	uiText(t, page.Locator(".flash-success"), "Connected via MCP OAuth")
	content, err := page.Content()
	require.NoError(t, err)
	require.NotContains(t, content, "ashby-oauth-token")
	require.NotContains(t, content, "ashby-refresh-token")
	saved := uiAshbySaved(t, app)
	require.True(t, saved.Enabled)
	require.Equal(t, "ashby-oauth-token", saved.Credentials["mcp_access_token"])
	require.Equal(t, "ashby-refresh-token", saved.Credentials["mcp_refresh_token"])
	require.Equal(t, "ashby-client", saved.Credentials["mcp_client_id"])
	require.Equal(t, fixture.server.URL, saved.Credentials["base_url"])

	uiAshbyRecheck(t, page, app, "Connected")
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Contains(t, uiAshbySearch(t, app), "ashby_get_pending_tasks")
	uiCapture(t, page, "ashby-connected-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiCapture(t, page, "ashby-connected-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiAshbyRecheck(t, page, app, "Connected")
	require.Contains(t, uiAshbySearch(t, app), "ashby_get_pending_tasks")

	// Disabled integrations drop out of MCP discovery.
	app.stop()
	saved = uiAshbySaved(t, app)
	saved.Enabled = false
	uiAshbyWrite(t, app, saved)
	app.start()
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiAshbySearch(t, app))
	uiCapture(t, page, "ashby-disabled-desktop")
}

func uiAshbyRecheck(t *testing.T, page playwright.Page, app *uiApp, want string) {
	t.Helper()
	uiNavigate(t, page, app.URL+"/integrations")
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all"}).Click())
	uiText(t, page.Locator(`a.integration-card[href="/integrations/ashby"]`), want)
}

func uiAshbyConfigPath(app *uiApp) string {
	return filepath.Join(app.home, ".config", "switchboard", "config.json")
}

func uiAshbySaved(t *testing.T, app *uiApp) *mcp.IntegrationConfig {
	t.Helper()
	data, err := os.ReadFile(uiAshbyConfigPath(app))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	ic, ok := saved.Integrations["ashby"]
	require.True(t, ok, "fresh installs include an ashby entry")
	return ic
}

func uiAshbySeed(t *testing.T, app *uiApp, creds mcp.Credentials) {
	t.Helper()
	ic := uiAshbySaved(t, app)
	ic.Enabled = true
	for key, value := range creds {
		ic.Credentials[key] = value
	}
	uiAshbyWrite(t, app, ic)
}

func uiAshbyWrite(t *testing.T, app *uiApp, ic *mcp.IntegrationConfig) {
	t.Helper()
	data, err := os.ReadFile(uiAshbyConfigPath(app))
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	integrations, ok := raw["integrations"].(map[string]any)
	require.True(t, ok)
	integrations["ashby"] = ic
	data, err = json.MarshalIndent(raw, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(uiAshbyConfigPath(app), data, 0600))
}

// uiAshbySearch searches for Ashby recruiting tools over the binary's MCP
// endpoint and, when found, executes a read and checks the fixture data.
func uiAshbySearch(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "pending recruiting tasks scorecards", "integration": "ashby", "limit": 5}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	var response struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &response))
	var names []string
	for _, tool := range response.Tools {
		names = append(names, tool.Name)
	}
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "ashby_get_pending_tasks")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "ashby_get_pending_tasks", "arguments": map[string]any{}}})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.False(t, result.IsError, text.Text)
	require.Contains(t, text.Text, "Fixture Candidate")
	return names
}
