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

const uiDiscordTools = 7

// uiDiscordMCP searches for Discord tools over MCP and, when they are
// enabled, reads the server list through execute.
func uiDiscordMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "discord-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "discord", "query": "discord server channel messages", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "discord_list_guilds")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "discord_list_guilds", "arguments": map[string]any{}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "Fixture Server")
	require.NotContains(t, text.Text, "SECRET_SENTINEL", "compaction must drop permissions")
	return names
}

func TestUIDiscord(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bot fixture-token" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"message":"401: Unauthorized","code":0}`)
			return
		}
		switch r.URL.Path {
		case "/api/v10/users/@me":
			fmt.Fprint(w, `{"id":"9","username":"fixture-bot","bot":true}`)
		case "/api/v10/users/@me/guilds":
			fmt.Fprint(w, `[{"id":"1","name":"Fixture Server","owner":false,"permissions":"SECRET_SENTINEL"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(provider.Close)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/discord"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiDiscordMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct {
		key, kind, hint string
		optional        bool
	}{
		{"bot_token", "password", "Developer Portal", false}, {"base_url", "text", "discord.com/api/v10", true},
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
	uiText(t, page.Locator("body"), fmt.Sprintf("Available Tools (%d)", uiDiscordTools))
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/api/v10"))
	require.NoError(t, form.Locator("#cred_bot_token").Fill("invalid"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	// A token pasted with its "Bot " prefix still works.
	require.NoError(t, form.Locator("#cred_bot_token").Fill("Bot fixture-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiDiscordMCP(t, app), uiDiscordTools)
	uiCapture(t, page, "discord-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "discord-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/discord")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiDiscordMCP(t, app), uiDiscordTools)
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiDiscordMCP(t, app))
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["discord"].Enabled)
	require.Equal(t, provider.URL+"/api/v10", saved.Integrations["discord"].Credentials["base_url"])
}
