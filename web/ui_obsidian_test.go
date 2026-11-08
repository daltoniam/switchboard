package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
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

const uiObsidianKey = "ui-fixture-obsidian-key"

// uiObsidianProvider mimics the Local REST API plugin: HTTPS with a
// self-signed certificate, a root that reports whether the key is valid, and
// vault routes that reject a bad key.
func uiObsidianProvider(t *testing.T) *httptest.Server {
	t.Helper()
	provider := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authenticated := request.Header.Get("Authorization") == "Bearer "+uiObsidianKey
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"status": "OK", "authenticated": authenticated})
			return
		}
		if !authenticated {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(writer, `{"message":"Authorization required.","errorCode":40101}`)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/vault/":
			_, _ = io.WriteString(writer, `{"files":["Inbox/","UI fixture note.md"]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/vault/UI fixture note.md":
			writer.Header().Set("Content-Type", "application/vnd.olrapi.note+json")
			_, _ = io.WriteString(writer, `{"path":"UI fixture note.md","content":"# Fixture\n\nUI fixture body.\n","frontmatter":{},"tags":["fixture"],"stat":{"ctime":1700000000000,"mtime":1700000500000,"size":30},"links":[],"backlinks":[],"unresolvedLinks":[]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(writer, `{"message":"Not Found","errorCode":40400}`)
		}
	}))
	t.Cleanup(provider.Close)
	return provider
}

// uiObsidianMCP searches for Obsidian tools and, when they are discoverable,
// lists the vault root and reads a note through execute.
func uiObsidianMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "obsidian-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "obsidian", "query": "obsidian vault notes", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "obsidian_list_files")
	require.Contains(t, names, "obsidian_read_note")
	for _, call := range []struct {
		tool string
		args map[string]any
		want string
	}{
		{"obsidian_list_files", map[string]any{}, "UI fixture note.md"},
		{"obsidian_read_note", map[string]any{"path": "UI fixture note.md"}, "UI fixture body."},
	} {
		result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": call.tool, "arguments": call.args}})
		require.NoError(t, err)
		require.False(t, result.IsError, call.tool)
		text, ok = result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		require.Contains(t, text.Text, call.want, call.tool)
	}
	return names
}

func TestUIObsidian(t *testing.T) {
	app := newUIApp(t)
	provider := uiObsidianProvider(t)
	// A text input drops line breaks, so paste the certificate as one line,
	// the way a user copying it from a browser would end up entering it.
	ca := strings.ReplaceAll(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})), "\n", "")
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/obsidian"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiObsidianMCP(t, app))
	require.NoError(t, card.Click())

	form := page.Locator("#integration-configuration")
	fields := []struct {
		key, kind, hint string
		optional        bool
	}{
		{"api_key", "password", "Local REST API", false},
		{"base_url", "text", "27124", true},
		{"ca_cert", "text", "PEM", true},
		{"insecure_skip_tls_verify", "text", "true", true},
	}
	for _, field := range fields {
		input := form.Locator("#cred_" + field.key)
		kind, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.kind, kind, field.key)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value, "fresh install must not prefill %s", field.key)
		hint, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, hint, field.hint, field.key)
		label, err := form.Locator(`label[for="cred_` + field.key + `"]`).TextContent()
		require.NoError(t, err)
		want := field.key
		if field.optional {
			want += " (optional)"
		}
		require.Equal(t, want, label)
	}
	uiText(t, page.Locator("body"), "Available Tools (13)")

	// The plugin's certificate is self-signed: without ca_cert the
	// connection must not be trusted.
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL))
	require.NoError(t, form.Locator("#cred_api_key").Fill(uiObsidianKey))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")

	require.NoError(t, form.Locator("#cred_ca_cert").Fill(ca))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")

	require.NoError(t, form.Locator("#cred_api_key").Fill("invalid-fixture-key"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_key").Fill(uiObsidianKey))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiObsidianMCP(t, app), 13)

	uiCapture(t, page, "obsidian-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	hidden, err := page.Locator("#sidebar").IsHidden()
	require.NoError(t, err)
	require.True(t, hidden)
	uiCapture(t, page, "obsidian-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/obsidian")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiObsidianMCP(t, app), 13)

	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiObsidianMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")

	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["obsidian"].Enabled)
	require.Equal(t, provider.URL, saved.Integrations["obsidian"].Credentials["base_url"])
	require.Equal(t, ca, saved.Integrations["obsidian"].Credentials["ca_cert"])
}
