package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/ashby"
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
		// The server registers http://localhost:<port>/...; the browser
		// sandbox only reaches 127.0.0.1, so the provider answers on the
		// same port and path through that address.
		appURL, err := url.Parse(app.URL)
		if err != nil {
			routeProblems = append(routeProblems, err.Error())
			_ = route.Abort()
			return
		}
		callback, err := url.Parse(authorize.Query().Get("redirect_uri"))
		want := "http://localhost:" + appURL.Port() + "/api/remote/ashby/oauth/callback"
		if err != nil || callback.String() != want {
			routeProblems = append(routeProblems, "redirect_uri="+authorize.Query().Get("redirect_uri")+", want "+want)
			_ = route.Abort()
			return
		}
		callback.Host = appURL.Host
		query := url.Values{"state": {authorize.Query().Get("state")}}
		if denied {
			query.Set("error", "access_denied")
		} else {
			query.Set("code", "fixture-code")
		}
		callback.RawQuery = query.Encode()
		location := callback.String()
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
		require.Empty(t, routeProblems, "authorize requests must carry prompt=consent, the /mcp/v1 resource, and the app callback")
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
	uiText(t, page.Locator("#ashby-api-key"), "Option 2: API key")
	uiText(t, page.Locator("body"), "Admin → API Keys")
	// The only inputs are the API key card's enabled toggle and key field,
	// and a fresh install pre-fills neither the key nor a sign-in.
	inputs, err := page.Locator("main input").Count()
	require.NoError(t, err)
	require.Equal(t, 2, inputs)
	keyInput := page.Locator("#cred_api_key")
	value, err := keyInput.InputValue()
	require.NoError(t, err)
	require.Empty(t, value)
	placeholder, err := keyInput.GetAttribute("placeholder")
	require.NoError(t, err)
	require.Equal(t, "Paste your Ashby API key", placeholder)
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

	// API key mode: the second option switches Ashby to the REST API, using
	// a local fixture in place of api.ashbyhq.com.
	api := ashbyAPIFixture(t, "fixture-api-key")
	app.stop()
	saved = uiAshbySaved(t, app)
	saved.Credentials["api_base_url"] = api.URL
	uiAshbyWrite(t, app, saved)
	app.start()
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiAshbySaveAPIKey(t, page, "wrong-fixture-key")
	uiText(t, page.Locator(".badge").First(), "Connection Error")
	uiText(t, page.Locator("#ashby-mode"), "API key")
	saved = uiAshbySaved(t, app)
	require.Equal(t, "wrong-fixture-key", saved.Credentials["api_key"])
	require.Empty(t, saved.Credentials["mcp_access_token"], "saving a key replaces the sign-in")
	require.Empty(t, saved.Credentials["mcp_refresh_token"])

	uiAshbySaveAPIKey(t, page, "fixture-api-key")
	uiText(t, page.Locator(".badge").First(), "Connected")
	content, err = page.Content()
	require.NoError(t, err)
	require.NotContains(t, content, "fixture-api-key", "the saved key is never rendered")
	placeholder, err = page.Locator("#cred_api_key").GetAttribute("placeholder")
	require.NoError(t, err)
	require.Equal(t, "Saved; leave blank to keep the current key", placeholder)
	uiNavigate(t, page, app.URL+"/integrations")
	apiCard := page.Locator(`a.integration-card[href="/integrations/ashby"]`)
	uiText(t, apiCard, fmt.Sprintf("%d tools", uiAshbyNativeToolCount(t)))
	uiText(t, apiCard, "Connected")
	uiAshbyAPISearch(t, app)
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiCapture(t, page, "ashby-api-key-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiCapture(t, page, "ashby-api-key-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/ashby/setup")
	uiText(t, page.Locator(".badge").First(), "Connected")
	uiAshbyAPISearch(t, app)

	// Signing in again replaces the API key, so only one mode is active.
	require.NoError(t, page.Locator("#ashby-oauth-start").Click())
	uiText(t, page.Locator(".flash-success"), "Connected via MCP OAuth")
	saved = uiAshbySaved(t, app)
	require.Empty(t, saved.Credentials["api_key"])
	require.Equal(t, "ashby-oauth-token", saved.Credentials["mcp_access_token"])
	uiText(t, page.Locator("#ashby-mode"), "Sign in with Ashby")
	require.Contains(t, uiAshbySearch(t, app), "ashby_get_pending_tasks")
}

func uiAshbySaveAPIKey(t *testing.T, page playwright.Page, key string) {
	t.Helper()
	require.NoError(t, page.Locator("#cred_api_key").Fill(key))
	enabled := page.Locator(`#ashby-api-key-form input[type="checkbox"][name="enabled"]`)
	checked, err := enabled.IsChecked()
	require.NoError(t, err)
	if !checked {
		require.NoError(t, page.Locator("#ashby-api-key-form .toggle").Click())
	}
	uiSave(t, page)
}

func uiAshbyNativeToolCount(t *testing.T) int {
	t.Helper()
	integration := ashby.New()
	require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"api_key": "count-only"}))
	return len(integration.Tools())
}

// uiAshbyAPISearch finds and runs a native read over the binary's MCP
// endpoint. The compacted result keeps the candidate's name but not the
// phone number the fixture returns.
func uiAshbyAPISearch(t *testing.T, app *uiApp) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "search candidates applicants by name", "integration": "ashby", "limit": 5}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "ashby_search_candidates")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "ashby_get_pending_tasks pending tasks scorecards", "integration": "ashby", "limit": 200}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	var listed struct {
		Tools struct {
			Columns []string `json:"columns"`
			Rows    [][]any  `json:"rows"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &listed))
	nameColumn := slices.Index(listed.Tools.Columns, "name")
	require.GreaterOrEqual(t, nameColumn, 0, "search response has no name column")
	require.Len(t, listed.Tools.Rows, uiAshbyNativeToolCount(t), "search must list exactly the native catalog")
	for _, row := range listed.Tools.Rows {
		require.NotEqual(t, "ashby_get_pending_tasks", row[nameColumn], "hosted-only tools are not offered in API key mode")
	}
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "ashby_search_candidates", "arguments": map[string]any{"name": "Fixture"}}})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.False(t, result.IsError, text.Text)
	require.Contains(t, text.Text, "Fixture Candidate")
	require.Contains(t, text.Text, "cand-fixture")
	require.NotContains(t, text.Text, "+1 555 0100", "compaction drops candidate phone numbers")
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
