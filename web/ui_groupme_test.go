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

const uiGroupMeTools = 10

// uiGroupMeMCP searches for GroupMe tools over MCP and, when they are
// enabled, reads the group list through execute.
func uiGroupMeMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "groupme-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "groupme", "query": "groupme group chat messages", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "groupme_list_groups")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "groupme_list_groups", "arguments": map[string]any{}}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "Weekend Crew")
	require.NotContains(t, text.Text, "SECRET_SENTINEL", "compaction must drop share_url")
	return names
}

func TestUIGroupMe(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Access-Token") != "fixture-token" || r.URL.Query().Has("token") {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"response":null,"meta":{"code":401,"errors":["unauthorized"]}}`)
			return
		}
		switch r.URL.Path {
		case "/v3/users/me":
			fmt.Fprint(w, `{"response":{"id":"9","user_id":"9","name":"Fixture User"},"meta":{"code":200}}`)
		case "/v3/groups":
			require.Equal(t, "memberships", r.URL.Query().Get("omit"))
			fmt.Fprint(w, `{"response":[{"id":"1","name":"Weekend Crew","type":"private","share_url":"https://groupme.com/join_group/SECRET_SENTINEL","messages":{"count":3,"last_message_id":"77","preview":{"nickname":"Ann","text":"see you"}}}],"meta":{"code":200}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(provider.Close)
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/groupme"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiGroupMeMCP(t, app))
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	for _, field := range []struct {
		key, kind, hint string
		optional        bool
	}{
		{"access_token", "password", "dev.groupme.com", false}, {"base_url", "text", "api.groupme.com", true},
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
	uiText(t, page.Locator("body"), fmt.Sprintf("Available Tools (%d)", uiGroupMeTools))
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/v3"))
	require.NoError(t, form.Locator("#cred_access_token").Fill("invalid"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_access_token").Fill("fixture-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiGroupMeMCP(t, app), uiGroupMeTools)
	uiCapture(t, page, "groupme-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "groupme-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/groupme")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.Len(t, uiGroupMeMCP(t, app), uiGroupMeTools)
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiGroupMeMCP(t, app))
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["groupme"].Enabled)
	require.Equal(t, provider.URL+"/v3", saved.Integrations["groupme"].Credentials["base_url"])
}
