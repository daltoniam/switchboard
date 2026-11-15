package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const uiGrafanaTools = 7

// uiGrafanaMCP searches for Grafana tools over MCP and, when they are
// enabled, searches dashboards through execute.
func uiGrafanaMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "grafana-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "grafana", "query": "grafana dashboards", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "grafana_search_dashboards")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "grafana_search_dashboards", "arguments": map[string]any{}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "Service Overview")
	require.NotContains(t, text.Text, "SECRET_SENTINEL", "compaction must drop folderUrl")
	return names
}

func TestUIGrafana(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"Invalid API key"}`)
			return
		}
		switch r.URL.Path {
		case "/grafana/api/org":
			fmt.Fprint(w, `{"id":1,"name":"Main Org."}`)
		case "/grafana/api/search":
			require.Equal(t, "dash-db", r.URL.Query().Get("type"))
			fmt.Fprint(w, `[{"id":7,"uid":"svc","title":"Service Overview","url":"/d/svc/service-overview","folderUid":"ops","folderTitle":"Ops","folderUrl":"/dashboards/f/SECRET_SENTINEL","tags":["prod"]}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(provider.Close)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/grafana"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiGrafanaMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct {
		key, kind, hint string
		optional        bool
	}{
		{"base_url", "text", "grafana.net", false}, {"service_account_token", "password", "Service account token", false},
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
	uiText(t, page.Locator("body"), fmt.Sprintf("Available Tools (%d)", uiGrafanaTools))
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/grafana"))
	require.NoError(t, form.Locator("#cred_service_account_token").Fill("invalid"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_service_account_token").Fill("fixture-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiGrafanaMCP(t, app), uiGrafanaTools)
	uiCapture(t, page, "grafana-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "grafana-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/grafana")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiGrafanaMCP(t, app), uiGrafanaTools)
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiGrafanaMCP(t, app))
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["grafana"].Enabled)
	require.Equal(t, provider.URL+"/grafana", saved.Integrations["grafana"].Credentials["base_url"])
	require.Equal(t, "fixture-token", saved.Integrations["grafana"].Credentials["service_account_token"])
}
