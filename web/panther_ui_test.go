package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/panther"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestUIPanther drives the development binary through both Panther setup
// options against local fixtures: API token mode against a fake REST API,
// then Sign in with Panther against a fake Remote MCP server with OAuth. No
// real Panther instance is contacted.
func TestUIPanther(t *testing.T) {
	app := newUIApp(t)
	api := pantherAPIFixture(t, "fixture-api-token")
	remote := newPantherOAuthFixture(t)
	page := uiPage(t)
	// Playwright runs route callbacks off the test goroutine, so problems
	// are recorded and checked here, and every request is answered.
	var routeMu sync.Mutex
	var routeProblems []string
	denied := false
	require.NoError(t, page.Route(remote.server.URL+"/oauth/authorize*", func(route playwright.Route) {
		routeMu.Lock()
		defer routeMu.Unlock()
		authorize, err := url.Parse(route.Request().URL())
		if err != nil {
			routeProblems = append(routeProblems, err.Error())
			_ = route.Abort()
			return
		}
		if got := authorize.Query().Get("resource"); got != remote.server.URL+"/mcp" {
			routeProblems = append(routeProblems, "resource="+got)
		}
		if authorize.Query().Has("scope") {
			routeProblems = append(routeProblems, "scope="+authorize.Query().Get("scope"))
		}
		if authorize.Query().Get("code_challenge_method") != "S256" {
			routeProblems = append(routeProblems, "missing PKCE")
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
		want := "http://localhost:" + appURL.Port() + "/api/remote/panther/oauth/callback"
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
		if err := route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(http.StatusFound), Headers: map[string]string{"Location": callback.String()}}); err != nil {
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
		require.Empty(t, routeProblems, "authorize requests must carry the /mcp resource, PKCE, no invented scope, and the app callback")
	})

	// Fresh install: the card routes to the setup page, which shows only
	// the instance URL, the enabled toggle, and the API token field, all
	// empty, and cannot sign in before an instance is saved.
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/panther"]`)
	uiText(t, card, "panther")
	uiText(t, card, fmt.Sprintf("%d tools", len(panther.New().Tools())))
	uiText(t, card, "Disabled")
	require.NoError(t, card.Click())
	require.NoError(t, page.WaitForURL(app.URL+"/integrations/panther/setup"))
	uiText(t, page.Locator("h1"), "Panther Setup")
	uiText(t, page.Locator(".badge").First(), "Not Connected")
	uiText(t, page.Locator("#panther-api-token"), "Option 2: API token")
	uiText(t, page.Locator("body"), "Settings → Developer Tools → API Tokens")
	inputs, err := page.Locator("main input:not([type=hidden])").Count()
	require.NoError(t, err)
	require.Equal(t, 3, inputs)
	for id, placeholder := range map[string]string{"#cred_instance_url": "https://acme.runpanther.net", "#cred_api_token": "Paste your Panther API token"} {
		value, err := page.Locator(id).InputValue()
		require.NoError(t, err)
		require.Empty(t, value, id)
		got, err := page.Locator(id).GetAttribute("placeholder")
		require.NoError(t, err)
		require.Equal(t, placeholder, got, id)
	}
	disabled, err := page.Locator("#panther-oauth-start").IsDisabled()
	require.NoError(t, err)
	require.True(t, disabled, "sign-in needs a saved instance URL")
	require.Empty(t, uiPantherSearch(t, app))
	uiCapture(t, page, "panther-fresh-desktop")

	// An API token without an instance is refused with a visible error.
	require.NoError(t, page.Locator("#cred_api_token").Fill("fixture-api-token"))
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save Configuration", Exact: playwright.Bool(true)}).Click())
	uiText(t, page.Locator(".flash-error").First(), "Save your Panther instance URL first")

	// Option 2: API token against the fixture REST API.
	uiPantherSaveInstance(t, page, "http://acme.runpanther.net")
	uiText(t, page.Locator(".flash-error").First(), "must use https")
	// One Save Configuration stores the instance typed above with the token.
	require.NoError(t, page.Locator("#cred_instance_url").Fill(api.URL+"/public/graphql"))
	uiPantherSaveAPIToken(t, page, "wrong-fixture-token")
	uiText(t, page.Locator(".badge").First(), "Connection Error")
	uiText(t, page.Locator(".flash-success"), "Panther did not accept the API token")
	uiText(t, page.Locator("#panther-mode"), "API token")
	value, err := page.Locator("#cred_instance_url").InputValue()
	require.NoError(t, err)
	require.Equal(t, api.URL, value, "the saved instance is normalized")

	// Enter in the token field submits the token form, not the instance form.
	require.NoError(t, page.Locator("#cred_api_token").Fill("fixture-api-token"))
	require.NoError(t, page.Locator("#cred_api_token").Press("Enter"))
	uiText(t, page.Locator(".flash-success"), "Configuration saved")
	uiText(t, page.Locator(".badge").First(), "Connected")
	content, err := page.Content()
	require.NoError(t, err)
	require.NotContains(t, content, "fixture-api-token", "the saved token is never rendered")
	placeholder, err := page.Locator("#cred_api_token").GetAttribute("placeholder")
	require.NoError(t, err)
	require.Equal(t, "Saved; leave blank to keep the current token", placeholder)
	saved := uiPantherSaved(t, app)
	require.True(t, saved.Enabled)
	require.Equal(t, "fixture-api-token", saved.Credentials["api_token"])
	require.Equal(t, api.URL, saved.Credentials["instance_url"])
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, page.Locator(`a.integration-card[href="/integrations/panther"]`), "Connected")
	uiPantherAPIRead(t, app)
	uiNavigate(t, page, app.URL+"/integrations/panther/setup")
	uiCapture(t, page, "panther-api-token-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiNavigate(t, page, app.URL+"/integrations/panther/setup")
	uiCapture(t, page, "panther-api-token-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiPantherRecheck(t, page, app, "Connected")
	uiPantherAPIRead(t, app)

	// Option 1: a new instance clears the API token, then Sign in with
	// Panther against the fixture Remote MCP server.
	uiNavigate(t, page, app.URL+"/integrations/panther/setup")
	uiPantherSaveInstance(t, page, remote.server.URL)
	uiText(t, page.Locator(".flash-success"), "Instance URL saved")
	uiText(t, page.Locator(".badge").First(), "Not Connected")
	saved = uiPantherSaved(t, app)
	require.Empty(t, saved.Credentials["api_token"], "a token for the old instance is cleared")

	setDenied(true)
	require.NoError(t, page.Locator("#panther-oauth-start").Click())
	uiText(t, page.Locator(".flash-error").First(), "access_denied")
	require.Empty(t, uiPantherSaved(t, app).Credentials["mcp_access_token"])

	setDenied(false)
	require.NoError(t, page.Locator("#panther-oauth-start").Click())
	uiText(t, page.Locator(".flash-success"), "Connected via MCP OAuth")
	content, err = page.Content()
	require.NoError(t, err)
	require.NotContains(t, content, "panther-oauth-token")
	require.NotContains(t, content, "panther-refresh-token")
	saved = uiPantherSaved(t, app)
	require.True(t, saved.Enabled)
	require.Equal(t, "panther-oauth-token", saved.Credentials["mcp_access_token"])
	require.Equal(t, "panther-refresh-token", saved.Credentials["mcp_refresh_token"])
	require.Equal(t, "panther-client", saved.Credentials["mcp_client_id"])
	require.Equal(t, remote.server.URL, saved.Credentials["instance_url"])

	uiPantherRecheck(t, page, app, "Connected")
	uiNavigate(t, page, app.URL+"/integrations/panther/setup")
	uiText(t, page.Locator(".badge").First(), "Connected")
	uiText(t, page.Locator("#panther-mode"), "Sign in with Panther")
	uiText(t, page.Locator("#panther-oauth-start"), "Re-authorize with Panther")
	require.Contains(t, uiPantherSearch(t, app), "panther_list_alerts")
	uiCapture(t, page, "panther-signed-in-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiNavigate(t, page, app.URL+"/integrations/panther/setup")
	uiCapture(t, page, "panther-signed-in-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiPantherRecheck(t, page, app, "Connected")
	require.Contains(t, uiPantherSearch(t, app), "panther_list_alerts")

	// Disabled integrations drop out of MCP discovery.
	app.stop()
	saved = uiPantherSaved(t, app)
	saved.Enabled = false
	uiPantherWrite(t, app, saved)
	app.start()
	uiNavigate(t, page, app.URL+"/integrations/panther/setup")
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiPantherSearch(t, app))
	uiCapture(t, page, "panther-disabled-desktop")
}

func uiPantherSaveInstance(t *testing.T, page playwright.Page, instance string) {
	t.Helper()
	require.NoError(t, page.Locator("#cred_instance_url").Fill(instance))
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Save Instance URL"}).Click())
	require.NoError(t, page.WaitForLoadState())
}

func uiPantherSaveAPIToken(t *testing.T, page playwright.Page, token string) {
	t.Helper()
	require.NoError(t, page.Locator("#cred_api_token").Fill(token))
	enabled := page.Locator(`#panther-api-token-form input[type="checkbox"][name="enabled"]`)
	checked, err := enabled.IsChecked()
	require.NoError(t, err)
	if !checked {
		require.NoError(t, page.Locator("#panther-api-token-form .toggle").Click())
	}
	uiSave(t, page)
}

func uiPantherRecheck(t *testing.T, page playwright.Page, app *uiApp, want string) {
	t.Helper()
	uiNavigate(t, page, app.URL+"/integrations")
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all"}).Click())
	uiText(t, page.Locator(`a.integration-card[href="/integrations/panther"]`), want)
}

func uiPantherSaved(t *testing.T, app *uiApp) *mcp.IntegrationConfig {
	t.Helper()
	data, err := os.ReadFile(uiAshbyConfigPath(app))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	ic, ok := saved.Integrations["panther"]
	require.True(t, ok, "fresh installs include a panther entry")
	return ic
}

func uiPantherWrite(t *testing.T, app *uiApp, ic *mcp.IntegrationConfig) {
	t.Helper()
	data, err := os.ReadFile(uiAshbyConfigPath(app))
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	integrations, ok := raw["integrations"].(map[string]any)
	require.True(t, ok)
	integrations["panther"] = ic
	data, err = json.MarshalIndent(raw, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(uiAshbyConfigPath(app), data, 0600))
}

func uiPantherSession(t *testing.T, app *uiApp) (*mcpsdk.ClientSession, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

// uiPantherToolNames reads tool names from a search response, which lists
// tools as objects or, for longer results, as columns and rows.
func uiPantherToolNames(t *testing.T, text string) []string {
	t.Helper()
	var objects struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal([]byte(text), &objects) == nil {
		var names []string
		for _, tool := range objects.Tools {
			names = append(names, tool.Name)
		}
		return names
	}
	var columnar struct {
		Tools struct {
			Columns []string `json:"columns"`
			Rows    [][]any  `json:"rows"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &columnar), text)
	column := slices.Index(columnar.Tools.Columns, "name")
	require.GreaterOrEqual(t, column, 0, "search response has no name column")
	var names []string
	for _, row := range columnar.Tools.Rows {
		names = append(names, fmt.Sprint(row[column]))
	}
	return names
}

func uiPantherCall(t *testing.T, session *mcpsdk.ClientSession, ctx context.Context, tool string, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: tool, Arguments: args})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.False(t, result.IsError, text.Text)
	return text.Text
}

// uiPantherSearch searches for Panther security alert tools over the
// binary's MCP endpoint and, when found, lists alerts.
func uiPantherSearch(t *testing.T, app *uiApp) []string {
	t.Helper()
	session, ctx := uiPantherSession(t, app)
	names := uiPantherToolNames(t, uiPantherCall(t, session, ctx, "search", map[string]any{"query": "panther security alerts triage", "integration": "panther", "limit": 5}))
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "panther_list_alerts")
	text := uiPantherCall(t, session, ctx, "execute", map[string]any{"tool_name": "panther_list_alerts", "arguments": map[string]any{}})
	require.Contains(t, text, "remote-alert")
	return names
}

// uiPantherAPIRead finds and runs the native alert list in API token mode;
// the compacted result keeps the alert's id and title.
func uiPantherAPIRead(t *testing.T, app *uiApp) {
	t.Helper()
	session, ctx := uiPantherSession(t, app)
	names := uiPantherToolNames(t, uiPantherCall(t, session, ctx, "search", map[string]any{"query": "panther SIEM security alerts", "integration": "panther", "limit": 5}))
	require.Contains(t, names, "panther_list_alerts")
	text := uiPantherCall(t, session, ctx, "execute", map[string]any{"tool_name": "panther_list_alerts", "arguments": map[string]any{"severity": "HIGH"}})
	require.Contains(t, text, "alert-fixture")
	require.Contains(t, text, "Fixture root login")
	text = uiPantherCall(t, session, ctx, "execute", map[string]any{"tool_name": "panther_get_permissions", "arguments": map[string]any{}})
	require.Contains(t, text, "AlertRead")
}
