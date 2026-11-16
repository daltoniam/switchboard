package web

import (
	"context"
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

func uiSnipeITMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "snipeit-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "Snipe-IT hardware asset inventory", "integration": "snipeit", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.ElementsMatch(t, []string{"snipeit_list_assets", "snipeit_get_asset", "snipeit_get_asset_by_tag"}, names)
	for _, name := range names {
		args := map[string]any{}
		if name == "snipeit_get_asset" {
			args["id"] = 7
		}
		if name == "snipeit_get_asset_by_tag" {
			args["tag"] = "IT-7"
		}
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": name, "arguments": args}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.Contains(t, text.Text, "Incident laptop")
		require.NotContains(t, text.Text, "SECRET_SENTINEL")
	}
	return names
}

func TestUISnipeIT(t *testing.T) {
	app := newUIApp(t)
	asset := `{"id":7,"name":"Incident laptop","asset_tag":"IT-7","serial":"SERIAL-7","model":{"id":2,"name":"Laptop"},"status_label":{"id":3,"name":"Ready"},"location":{"id":4,"name":"Office"},"assigned_to":{"id":5,"type":"user","email":"SECRET_SENTINEL"},"notes":"SECRET_SENTINEL","custom_fields":{"password":"SECRET_SENTINEL"}}`
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ui-snipeit-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/inventory/api/v1/hardware":
			_, _ = io.WriteString(w, `{"total":1,"rows":[`+asset+`]}`)
		case "/inventory/api/v1/hardware/7", "/inventory/api/v1/hardware/bytag/IT-7":
			_, _ = io.WriteString(w, asset)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}))
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/snipeit"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiSnipeITMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS Snipe-IT v8.8.0"}, {"token", "password", "assets.view only"}, {"ca_cert", "text", "PEM CA certificate"},
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
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/inventory"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(cert, "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_token").Fill("invalid-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("ui-snipeit-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiSnipeITMCP(t, app), 3)
	uiCapture(t, page, "snipeit-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "snipeit-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/snipeit")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiSnipeITMCP(t, app), 3)
	require.NoError(t, form.Locator("#cred_token").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("ui-snipeit-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiSnipeITMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
}
