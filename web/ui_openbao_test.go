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

func uiOpenBaoMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "openbao-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "openbao", "query": "openbao secret metadata", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "openbao_list_secret_keys")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "openbao_list_secret_keys", "arguments": map[string]any{"limit": 1}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "database")
	require.NotContains(t, text.Text, "SECRET_SENTINEL")
	return names
}

func TestUIOpenBao(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Header.Get("X-Vault-Token") != "metadata-reader" {
			writer.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(writer, `{"errors":["permission denied"]}`)
			return
		}
		switch request.URL.Path {
		case "/v1/sys/internal/ui/mounts/secret/metadata/approved/app/":
			_, _ = io.WriteString(writer, `{"data":{"type":"kv","path":"secret/","options":{"version":"2"}}}`)
		case "/v1/secret/metadata/approved/app/":
			_, _ = io.WriteString(writer, `{"data":{"keys":["database"],"key_info":{"password":"SECRET_SENTINEL"}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	ca := strings.ReplaceAll(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})), "\n", `\n`)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/openbao"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiOpenBaoMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	fields := []struct{ key, kind, hint string }{
		{"base_url", "text", "https://bao.example.com"},
		{"token", "password", "Short-lived"},
		{"mount", "text", "KV v2"},
		{"path_prefix", "text", "narrow prefix"},
		{"ca_cert", "text", "PEM"},
	}
	for _, field := range fields {
		input := form.Locator("#cred_" + field.key)
		kind, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.kind, kind)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		hint, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, hint, field.hint)
		label, err := form.Locator(`label[for="cred_` + field.key + `"]`).TextContent()
		require.NoError(t, err)
		if field.key == "ca_cert" {
			require.Equal(t, "ca_cert (optional)", label)
		} else {
			require.Equal(t, field.key, label)
		}
	}
	uiText(t, page.Locator("body"), "Available Tools (3)")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL))
	require.NoError(t, form.Locator("#cred_mount").Fill("secret"))
	require.NoError(t, form.Locator("#cred_path_prefix").Fill("approved/app"))
	require.NoError(t, form.Locator("#cred_ca_cert").Fill(ca))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("invalid-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_token").Fill("metadata-reader"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiOpenBaoMCP(t, app), 3)
	uiCapture(t, page, "openbao-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	hidden, err := page.Locator("#sidebar").IsHidden()
	require.NoError(t, err)
	require.True(t, hidden)
	uiCapture(t, page, "openbao-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/openbao")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiOpenBaoMCP(t, app), 3)
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiOpenBaoMCP(t, app))
}
