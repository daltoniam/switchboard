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

const uiGatusEndpoint = `{"key":"production_api","name":"Incident API","group":"production","results":[{"timestamp":"2026-11-18T11:00:00Z","success":false,"status":503,"duration":2500000,"errors":["SECRET_SENTINEL"],"hostname":"SECRET_SENTINEL","conditionResults":[{"condition":"SECRET_SENTINEL"}]}],"events":[{"type":"SECRET_SENTINEL"}]}`

func uiGatusMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "gatus-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	catalog, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"limit": 0}})
	require.NoError(t, err)
	require.False(t, catalog.IsError)
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "Gatus monitored endpoint health history uptime", "integration": "gatus", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.ElementsMatch(t, []string{"gatus_list_endpoints", "gatus_get_endpoint", "gatus_get_uptime"}, names)
	var discovered struct {
		Endpoints []struct {
			Key string `json:"key"`
		} `json:"endpoints"`
	}
	for _, name := range []string{"gatus_list_endpoints", "gatus_get_endpoint", "gatus_get_uptime"} {
		args := map[string]any{}
		if name != "gatus_list_endpoints" {
			require.NotEmpty(t, discovered.Endpoints)
			args["key"] = discovered.Endpoints[0].Key
		}
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": name, "arguments": args}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.NotContains(t, text.Text, "SECRET_SENTINEL")
		switch name {
		case "gatus_list_endpoints":
			require.Contains(t, text.Text, "Incident API")
			require.NoError(t, json.Unmarshal([]byte(text.Text), &discovered))
		case "gatus_get_uptime":
			require.Contains(t, text.Text, "0.75")
		default:
			require.Contains(t, text.Text, "Incident API")
		}
	}
	for _, script := range []string{
		`var inventory = api.call('gatus_list_endpoints', {}); JSON.stringify(api.call('gatus_get_endpoint', {key: inventory.endpoints[0].key}));`,
		`var inventory = api.call('gatus_list_endpoints', {}); JSON.stringify(api.call('gatus_get_uptime', {key: inventory.endpoints[0].key}));`,
	} {
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"script": script}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.Contains(t, text.Text, "production_api")
		require.NotContains(t, text.Text, "SECRET_SENTINEL")
	}
	return names
}

func TestUIGatus(t *testing.T) {
	app := newUIApp(t)
	t.Logf("isolated development URL: %s", app.URL)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "reader" || password != "ui-gatus-password" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/gatus/api/v1/endpoints/statuses":
			_, _ = io.WriteString(w, "["+uiGatusEndpoint+"]")
		case "/gatus/api/v1/endpoints/production_api/statuses":
			_, _ = io.WriteString(w, uiGatusEndpoint)
		case "/gatus/api/v1/endpoints/production_api/uptimes/24h":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "0.750000")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/gatus"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiGatusMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS Gatus v5.37.0"}, {"username", "text", "installation-wide"},
		{"password", "password", "Basic password"}, {"ca_cert", "text", "PEM CA certificate"},
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
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/gatus"))
	require.NoError(t, form.Locator("#cred_username").Fill("reader"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(string(cert), "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_password").Fill("invalid-password"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill("ui-gatus-password"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiGatusMCP(t, app), 3)
	uiCapture(t, page, "gatus-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "gatus-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/gatus")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiGatusMCP(t, app), 3)
	require.NoError(t, form.Locator("#cred_password").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill("ui-gatus-password"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiGatusMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
}
