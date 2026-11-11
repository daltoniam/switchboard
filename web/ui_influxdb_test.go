package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const uiInfluxOrg = "0123456789abcdef"

func uiInfluxMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "influxdb-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "influxdb", "query": "influxdb task retention downsampling", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "influxdb_list_buckets")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "influxdb_list_buckets", "arguments": map[string]any{}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "retention-fixture")
	require.NotContains(t, text.Text, "SECRET_SENTINEL")
	return names
}

func TestUIInfluxDB(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Token fixture-token" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"invalid token"}`)
			return
		}
		switch r.URL.Path {
		case "/health":
			fmt.Fprint(w, `{"status":"pass","version":"v2.8.0"}`)
		case "/api/v2/tasks":
			require.Equal(t, uiInfluxOrg, r.URL.Query().Get("orgID"))
			fmt.Fprint(w, `{"tasks":[]}`)
		case "/api/v2/buckets":
			fmt.Fprintf(w, `{"buckets":[{"id":"1111111111111111","orgID":"%s","name":"retention-fixture","retentionRules":[{"type":"expire","everySeconds":86400}],"description":"SECRET_SENTINEL"}]}`, uiInfluxOrg)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(provider.Close)
	ca := strings.ReplaceAll(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})), "\n", `\n`)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/influxdb"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiInfluxMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct {
		key, kind, hint string
		optional        bool
	}{
		{"base_url", "text", "OSS 2.8", false}, {"org_id", "text", "hexadecimal", false}, {"api_token", "password", "read tasks", false}, {"ca_cert", "text", "PEM", true},
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
	uiText(t, page.Locator("body"), "Available Tools (5)")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL))
	require.NoError(t, form.Locator("#cred_org_id").Fill(uiInfluxOrg))
	require.NoError(t, form.Locator("#cred_api_token").Fill("fixture-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(ca))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator("#cred_api_token").Fill("invalid"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill("fixture-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiInfluxMCP(t, app), 5)
	uiCapture(t, page, "influxdb-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "influxdb-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/influxdb")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiInfluxMCP(t, app), 5)
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiInfluxMCP(t, app))
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["influxdb"].Enabled)
	require.Equal(t, provider.URL, saved.Integrations["influxdb"].Credentials["base_url"])
}
