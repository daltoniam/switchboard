package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const uiAuthentikUser = `{"pk":42,"username":"ada","is_active":false,"last_login":null,"groups":["28e38d9e-72e6-4c8b-9c2f-10f58456caa1"],"email":"SECRET_SENTINEL","attributes":{"secret":"SECRET_SENTINEL"},"groups_obj":[{"name":"SECRET_SENTINEL"}],"roles_obj":[{"name":"SECRET_SENTINEL"}]}`

func uiAuthentikMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "authentik-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "authentik user identity active group applications access", "integration": "authentik", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.ElementsMatch(t, []string{"authentik_list_users", "authentik_get_user", "authentik_list_user_applications"}, names)
	var discovered struct {
		Page struct {
			Results []struct {
				PK int64 `json:"pk"`
			} `json:"results"`
		} `json:"page"`
	}
	for _, name := range []string{"authentik_list_users", "authentik_get_user", "authentik_list_user_applications"} {
		args := map[string]any{}
		if name != "authentik_list_users" {
			require.NotEmpty(t, discovered.Page.Results)
			args["user_id"] = discovered.Page.Results[0].PK
		}
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": name, "arguments": args}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.NotContains(t, text.Text, "SECRET_SENTINEL")
		if name == "authentik_list_users" {
			require.NoError(t, json.Unmarshal([]byte(text.Text), &discovered))
		}
		if name == "authentik_list_user_applications" {
			require.Contains(t, text.Text, "Payroll")
		} else {
			require.Contains(t, text.Text, "ada")
		}
	}
	for _, script := range []string{
		`var users = api.call('authentik_list_users', {}); JSON.stringify(api.call('authentik_get_user', {user_id: users.page.results[0].pk}));`,
		`var users = api.call('authentik_list_users', {}); JSON.stringify(api.call('authentik_list_user_applications', {user_id: users.page.results[0].pk}));`,
	} {
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"script": script}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.NotContains(t, text.Text, "SECRET_SENTINEL")
	}
	return names
}

func TestUIAuthentik(t *testing.T) {
	app := newUIApp(t)
	t.Logf("isolated development URL: %s", app.URL)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ui-authentik-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		page := `{"pagination":{"next":0,"previous":0,"current":1,"total_pages":1,"count":1},"results":[`
		switch r.URL.Path {
		case "/tenant/api/v3/core/users/":
			require.Equal(t, "false", r.URL.Query().Get("include_groups"))
			require.Equal(t, "false", r.URL.Query().Get("include_roles"))
			_, _ = io.WriteString(w, page+uiAuthentikUser+`]}`)
		case "/tenant/api/v3/core/users/42/":
			_, _ = io.WriteString(w, uiAuthentikUser)
		case "/tenant/api/v3/core/applications/":
			require.Equal(t, "42", r.URL.Query().Get("for_user"))
			require.Equal(t, "false", r.URL.Query().Get("superuser_full_list"))
			_, _ = io.WriteString(w, page+`{"pk":"28e38d9e-72e6-4c8b-9c2f-10f58456caa2","name":"Payroll","slug":"payroll","provider_obj":{"secret":"SECRET_SENTINEL"},"launch_url":"SECRET_SENTINEL"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/authentik"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiAuthentikMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS authentik 2026.8.3"}, {"token", "password", "non-superuser API-intent"}, {"ca_cert", "text", "PEM CA certificate"},
	} {
		input := form.Locator("#cred_" + field.key)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.inputType, inputType)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, placeholder, field.placeholder)
	}
	uiText(t, form.Locator(`label[for="cred_ca_cert"]`), "optional")
	uiText(t, page.Locator("body"), "Available Tools (3)")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/tenant"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(string(cert), "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_token").Fill("invalid-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("ui-authentik-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiAuthentikMCP(t, app), 3)
	uiCapture(t, page, "authentik-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "authentik-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/authentik")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiAuthentikMCP(t, app), 3)
	require.NoError(t, form.Locator("#cred_token").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("ui-authentik-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiAuthentikMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
}
