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

func uiNexusMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "nexus-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "Nexus private package component repository", "integration": "nexus", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.ElementsMatch(t, []string{"nexus_list_components", "nexus_search_components", "nexus_get_component"}, names)
	for _, name := range names {
		args := map[string]any{"repository": "maven-releases"}
		if name == "nexus_get_component" {
			args = map[string]any{"id": "component-1"}
		}
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": name, "arguments": args}})
		require.NoError(t, err)
		require.False(t, result.IsError)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.Contains(t, text.Text, "demo")
		require.NotContains(t, text.Text, "SECRET_SENTINEL")
	}
	return names
}

func TestUINexus(t *testing.T) {
	app := newUIApp(t)
	component := `{"id":"component-1","repository":"maven-releases","format":"maven2","group":"org.example","name":"demo","version":"1.0","downloadUrl":"SECRET_SENTINEL","assets":[{"id":"asset-1","path":"demo.pom","contentType":"application/xml","fileSize":42,"lastModified":"2026-11-01T00:00:00Z"}]}`
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "reader" || password != "ui-nexus-password" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/prefix/service/rest/v1/components", "/prefix/service/rest/v1/search":
			_, _ = io.WriteString(w, `{"items":[`+component+`],"continuationToken":null}`)
		case "/prefix/service/rest/v1/components/component-1":
			_, _ = io.WriteString(w, component)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw}))
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/nexus"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiNexusMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS Nexus"}, {"username", "text", "non-admin reader"},
		{"password", "password", "Reader password"}, {"repositories", "text", "approved repository"}, {"ca_cert", "text", "PEM CA certificate"},
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
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/prefix"))
	require.NoError(t, form.Locator("#cred_username").Fill("reader"))
	require.NoError(t, form.Locator("#cred_repositories").Fill("maven-releases"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(strings.ReplaceAll(cert, "\n", `\n`)))
	require.NoError(t, form.Locator("#cred_password").Fill("invalid-password"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill("ui-nexus-password"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiNexusMCP(t, app), 3)
	uiCapture(t, page, "nexus-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "nexus-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/nexus")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiNexusMCP(t, app), 3)
	require.NoError(t, form.Locator("#cred_password").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_password").Fill("ui-nexus-password"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiNexusMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
}
