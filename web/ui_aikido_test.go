package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/aikido"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const (
	uiAikidoClientID = "AIK_CLIENT_ui"
	uiAikidoSecret   = "ui-aikido-secret"
	uiAikidoToken    = "ui-aikido-access-token"
)

func uiAikidoCall(t *testing.T, ctx context.Context, session *mcpsdk.ClientSession, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: args})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.False(t, result.IsError, text.Text)
	for _, secret := range []string{uiAikidoSecret, uiAikidoToken, "SECRET_SENTINEL"} {
		require.NotContains(t, text.Text, secret)
	}
	return text.Text
}

func uiAikidoMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "aikido-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "aikido security vulnerabilities issues", "integration": "aikido", "limit": 10}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "aikido_list_issue_groups")

	groups := uiAikidoCall(t, ctx, session, map[string]any{"tool_name": "aikido_list_issue_groups", "arguments": map[string]any{"filter_issue_type": "open_source"}})
	require.Contains(t, groups, "lodash prototype pollution")
	require.Contains(t, groups, `"has_next_page":false`)
	require.NotContains(t, groups, "Upgrade lodash", "list compaction leaves how_to_fix to the get tool")
	issue := uiAikidoCall(t, ctx, session, map[string]any{"tool_name": "aikido_get_issue_group", "arguments": map[string]any{"issue_group_id": 42}})
	require.Contains(t, issue, "Upgrade lodash")
	leaks := uiAikidoCall(t, ctx, session, map[string]any{"script": `JSON.stringify(api.call('aikido_list_domain_data_leaks', {domain_id: 7}))`})
	require.Contains(t, leaks, "dev@example.com")
	require.Contains(t, leaks, "[redacted]", "scripts get raw output, so redaction happens in the handler")
	return names
}

func TestUIAikido(t *testing.T) {
	app := newUIApp(t)
	t.Logf("isolated development URL: %s", app.URL)
	var tokens atomic.Int32
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte(uiAikidoClientID+":"+uiAikidoSecret))
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/oauth/token" {
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != basic || !strings.Contains(string(body), "client_credentials") {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"invalid_client","error_description":"The provided client credentials are invalid"}`)
				return
			}
			tokens.Add(1)
			_, _ = fmt.Fprintf(w, `{"access_token":%q,"expires_in":3600,"token_type":"bearer"}`, uiAikidoToken)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+uiAikidoToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/public/v1/workspace":
			_, _ = io.WriteString(w, `{"id":1,"name":"UI Workspace","linked_provider":"github"}`)
		case "GET /api/public/v1/open-issue-groups":
			if r.URL.Query().Get("filter_issue_type") != "open_source" || r.URL.Query().Get("page") != "0" || r.URL.Query().Get("per_page") != "20" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"expected filter_issue_type=open_source on page 0 of 20"}`)
				return
			}
			w.Header().Set("X-Has-Next-Page", "false")
			_, _ = io.WriteString(w, `[{"id":42,"title":"lodash prototype pollution","type":"open_source","severity":"high","severity_score":80,"group_status":"new","related_cve_ids":["CVE-2021-23337"],"locations":[{"id":3,"name":"web-app","type":"code_repository"}],"how_to_fix":"Upgrade lodash to 4.17.21"}]`)
		case "GET /api/public/v1/issues/groups/42":
			_, _ = io.WriteString(w, `{"id":42,"title":"lodash prototype pollution","type":"open_source","severity":"high","group_status":"new","how_to_fix":"Upgrade lodash to 4.17.21","locations":[{"id":3,"name":"web-app","type":"code_repository"}]}`)
		case "GET /api/public/v1/domains/7/dataleaks":
			_, _ = io.WriteString(w, `{"dataleaks":[{"id":"l1","email":"dev@example.com","password":"SECRET_SENTINEL","status":"open"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	toolCount := len(aikido.New().Tools())

	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/aikido"]`)
	uiText(t, card, "Disabled")
	require.Empty(t, uiAikidoMCP(t, app))
	require.NoError(t, card.Click())
	steps := page.Locator("#integration-setup-steps")
	uiText(t, steps, "Settings > Integrations > REST API")
	uiText(t, steps, "basics:read")
	form := page.Locator("#integration-configuration")
	for _, field := range []struct {
		key, kind, hint string
		optional        bool
	}{
		{"client_id", "text", "AIK_CLIENT_", false},
		{"client_secret", "password", "shown once", false},
		{"region", "text", "us, au or me", true},
		{"base_url", "text", "app.us.aikido.dev", true},
	} {
		input := form.Locator("#cred_" + field.key)
		kind, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.kind, kind, field.key)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value, field.key)
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
	uiText(t, page.Locator("body"), fmt.Sprintf("Available Tools (%d)", toolCount))

	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL))
	require.NoError(t, form.Locator("#cred_client_id").Fill(uiAikidoClientID))
	require.NoError(t, form.Locator("#cred_client_secret").Fill("wrong-secret"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.Zero(t, tokens.Load(), "a wrong secret never gets a token")

	require.NoError(t, form.Locator("#cred_client_secret").Fill(uiAikidoSecret))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NotEmpty(t, uiAikidoMCP(t, app))
	uiCapture(t, page, "aikido-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "aikido-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/aikido")
	uiText(t, page.Locator(".badge").First(), "Connected")
	require.NotEmpty(t, uiAikidoMCP(t, app))

	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiAikidoMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")

	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["aikido"].Enabled)
	require.Equal(t, uiAikidoClientID, saved.Integrations["aikido"].Credentials["client_id"])
	require.Equal(t, provider.URL, saved.Integrations["aikido"].Credentials["base_url"])
}
