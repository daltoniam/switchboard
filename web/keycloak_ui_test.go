package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

func keycloakUISearch(t *testing.T, app *uiApp, enabled bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "keycloak-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	call := func(name string, args map[string]any) string {
		result, callErr := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		require.NoError(t, callErr)
		require.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.NotContains(t, text.Text, "NEVER-RETURN")
		return text.Text
	}
	var response struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	text := call("search", map[string]any{"query": "keycloak", "integration": "keycloak", "limit": 10})
	require.NoError(t, json.Unmarshal([]byte(text), &response))
	if !enabled {
		require.Empty(t, response.Tools)
		return
	}
	require.Len(t, response.Tools, 4)
	for _, tool := range response.Tools {
		require.True(t, strings.HasPrefix(tool.Name, "keycloak_"))
	}
	cases := []struct{ query, expected string }{
		{"find disabled identity accounts", "keycloak_list_users"},
		{"failed login audit events", "keycloak_list_events"},
		{"search groups", "keycloak_list_groups"},
	}
	for _, test := range cases {
		text = call("search", map[string]any{"query": test.query, "integration": "keycloak", "limit": 3})
		require.Contains(t, text, test.expected)
	}
	text = call("execute", map[string]any{"tool_name": "keycloak_list_users", "arguments": map[string]any{"max": 2}})
	require.Contains(t, text, "ui-alice")
	text = call("execute", map[string]any{"script": `var users = api.call("keycloak_list_users", {max:2}); api.call("keycloak_get_user", {user_id:users.items[0].id});`})
	require.Contains(t, text, "ui-alice")
	text = call("execute", map[string]any{"tool_name": "keycloak_list_groups", "arguments": map[string]any{}})
	require.Contains(t, text, "support-group")
	text = call("execute", map[string]any{"tool_name": "keycloak_list_events", "arguments": map[string]any{"date_from": "2026-10-01", "date_to": "2026-10-01"}})
	require.Contains(t, text, "LOGIN_ERROR")
	require.Contains(t, text, "events_enabled")
}

func TestUIKeycloakLifecycle(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/realms/support/protocol/openid-connect/token":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			require.Equal(t, "reader", r.Form.Get("client_id"))
			require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
			if r.Form.Get("client_secret") != "ui-fixture-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":"invalid_client"}`)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"ui-fixture-token","token_type":"Bearer","expires_in":60}`)
		case "/auth/admin/realms/support/users":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer ui-fixture-token", r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `[{"id":"user-1","username":"ui-alice","enabled":false,"credentials":[{"value":"NEVER-RETURN"}],"attributes":{"secret":"NEVER-RETURN"}}]`)
		case "/auth/admin/realms/support/users/user-1":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer ui-fixture-token", r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `{"id":"user-1","username":"ui-alice","enabled":false,"credentials":[{"value":"NEVER-RETURN"}]}`)
		case "/auth/admin/realms/support/groups":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer ui-fixture-token", r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `[{"id":"group-1","name":"support-group","path":"/support-group","attributes":{"secret":"NEVER-RETURN"}}]`)
		case "/auth/admin/realms/support/events/config":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer ui-fixture-token", r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, `{"eventsEnabled":true}`)
		case "/auth/admin/realms/support/events":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer ui-fixture-token", r.Header.Get("Authorization"))
			require.Equal(t, []string{"LOGIN", "LOGIN_ERROR"}, r.URL.Query()["type"])
			_, _ = io.WriteString(w, `[{"time":1790812800000,"type":"LOGIN_ERROR","userId":"user-1","details":{"secret":"NEVER-RETURN"},"ipAddress":"NEVER-RETURN"}]`)
		default:
			t.Errorf("unexpected fixture route: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	page := uiPage(t)
	t.Logf("isolated Keycloak browser URL: %s", app.URL)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/keycloak"]`)
	uiText(t, card, "Disabled")
	keycloakUISearch(t, app, false)
	require.NoError(t, card.Click())
	uiText(t, page.Locator("h1"), "keycloak")
	form := page.Locator("#integration-configuration")
	for _, key := range []string{"base_url", "realm", "client_id", "client_secret"} {
		input := form.Locator("#cred_" + key)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		if key == "client_secret" {
			require.Equal(t, "password", inputType)
		} else {
			require.Equal(t, "text", inputType)
		}
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.NotEmpty(t, placeholder)
	}
	uiText(t, page.Locator("body"), "Available Tools (4)")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/auth"))
	require.NoError(t, form.Locator("#cred_realm").Fill("support"))
	require.NoError(t, form.Locator("#cred_client_id").Fill("reader"))
	require.NoError(t, form.Locator("#cred_client_secret").Fill("ui-fixture-secret"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	keycloakUISearch(t, app, true)
	uiCapture(t, page, "keycloak-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Close sidebar", Exact: playwright.Bool(true)}).Click())
	require.NoError(t, page.Locator("#sidebar").WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateHidden}))
	overflows, err := page.Evaluate(`document.documentElement.scrollWidth > window.innerWidth`)
	require.NoError(t, err)
	require.Equal(t, false, overflows)
	uiCapture(t, page, "keycloak-mobile")
	_, err = page.Reload()
	require.NoError(t, err)
	uiText(t, page.Locator(".badge").First(), "Connected")
	app.restart()
	t.Logf("restarted isolated Keycloak browser URL: %s", app.URL)
	uiNavigate(t, page, app.URL+"/integrations/keycloak")
	uiText(t, page.Locator(".badge").First(), "Connected")
	keycloakUISearch(t, app, true)
	require.NoError(t, form.Locator("#cred_client_secret").Fill("invalid-fixture-secret"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	uiCapture(t, page, "keycloak-invalid")
	require.NoError(t, form.Locator("#cred_client_secret").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_client_secret").Fill("ui-fixture-secret"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	uiNavigate(t, page, app.URL+"/integrations")
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all"}).Click())
	uiText(t, card, "Connected")
	require.NoError(t, card.Click())
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	keycloakUISearch(t, app, false)
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	keycloakUISearch(t, app, false)
	uiCapture(t, page, "keycloak-disabled")
}
