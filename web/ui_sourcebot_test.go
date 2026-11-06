package web

import (
	"context"
	"encoding/json"
	"io"
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

const uiSourcebotKey = "sourcebot-0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab"

// uiSourcebotMCP searches for Sourcebot tools on the running binary and,
// when they are discoverable, runs a read-only code search against the fixture.
func uiSourcebotMCP(t *testing.T, app *uiApp) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "sourcebot code search", "integration": "sourcebot", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil
	}
	require.Contains(t, names, "sourcebot_search_code")

	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{
		"tool_name": "sourcebot_search_code",
		"arguments": map[string]any{"query": "ComputeInvoiceTotal", "repos": []any{"github.com/acme/api"}},
	}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok = result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "internal/billing/invoice.go")
	require.Contains(t, text.Text, "func ComputeInvoiceTotal")
	return names
}

func TestUISourcebot(t *testing.T) {
	app := newUIApp(t)
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Sourcebot-Api-Key") != uiSourcebotKey {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(writer, `{"statusCode":401,"errorCode":"NOT_AUTHENTICATED","message":"Not authenticated"}`)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/repos":
			writer.Header().Set("X-Total-Count", "1")
			_, _ = io.WriteString(writer, `[{"codeHostType":"github","repoId":1,"repoName":"github.com/acme/api","webUrl":"https://sourcebot.example.com/browse/github.com/acme/api","isFork":false,"isArchived":false}]`)
		case "/api/search":
			var body struct {
				Query string `json:"query"`
			}
			if json.NewDecoder(request.Body).Decode(&body) != nil || body.Query != `"ComputeInvoiceTotal" (repo:github\.com/acme/api)` {
				http.Error(writer, `{"statusCode":400,"errorCode":"INVALID_REQUEST_BODY","message":"unexpected query"}`, http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(writer, `{"stats":{"actualMatchCount":1,"totalMatchCount":1,"fileCount":1},"files":[{"fileName":{"text":"internal/billing/invoice.go","matchRanges":[]},"repository":"github.com/acme/api","repositoryId":1,"language":"Go","webUrl":"https://sourcebot.example.com/browse/github.com/acme/api/-/blob/internal/billing/invoice.go","chunks":[{"content":"func ComputeInvoiceTotal(items []int) int {\n","matchRanges":[{"start":{"byteOffset":5,"column":6,"lineNumber":4},"end":{"byteOffset":24,"column":25,"lineNumber":4}}],"contentStart":{"byteOffset":0,"column":1,"lineNumber":4}}]}],"repositoryInfo":[],"isSearchExhaustive":true}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)

	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/sourcebot"]`)
	uiText(t, card, "Disabled")
	logo, err := card.Locator(`.integration-icon[data-integration="sourcebot"]`).Evaluate(`icon => getComputedStyle(icon).getPropertyValue("--un-icon")`, nil)
	require.NoError(t, err)
	require.Contains(t, logo, "data:image/svg+xml", "Sourcebot card must show its logo")
	require.Empty(t, uiSourcebotMCP(t, app))
	require.NoError(t, card.Click())

	form := page.Locator("#integration-configuration")
	fields := []struct {
		key         string
		inputType   string
		label       string
		placeholder string
	}{
		{key: "base_url", inputType: "text", label: "base_url", placeholder: "https://sourcebot.example.com"},
		{key: "api_key", inputType: "password", label: "api_key (optional)", placeholder: "Settings → API Keys"},
	}
	for _, field := range fields {
		input := form.Locator("#cred_" + field.key)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.inputType, inputType, field.key)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value, "fresh install must not pre-seed %s", field.key)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, placeholder, field.placeholder, field.key)
		label, err := form.Locator(`label[for="cred_` + field.key + `"]`).TextContent()
		require.NoError(t, err)
		require.Equal(t, field.label, label)
	}
	uiText(t, page.Locator("body"), "Available Tools (9)")

	key := form.Locator("#cred_api_key")
	require.NoError(t, form.Locator("#cred_base_url").Fill(provider.URL+"/"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")

	require.NoError(t, key.Fill("sourcebot-wrong"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")

	require.NoError(t, key.Fill(uiSourcebotKey))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	names := uiSourcebotMCP(t, app)
	require.Len(t, names, 9)
	uiCapture(t, page, "sourcebot-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	uiCapture(t, page, "sourcebot-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/sourcebot")
	uiText(t, page.Locator(".badge").First(), "Connected")
	value, err := key.InputValue()
	require.NoError(t, err)
	require.Equal(t, uiSourcebotKey, value)
	require.NotEmpty(t, uiSourcebotMCP(t, app))

	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	require.Empty(t, uiSourcebotMCP(t, app))
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	uiCapture(t, page, "sourcebot-disabled")

	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["sourcebot"].Enabled)
	require.Equal(t, provider.URL+"/", saved.Integrations["sourcebot"].Credentials["base_url"])
	require.Equal(t, uiSourcebotKey, saved.Integrations["sourcebot"].Credentials["api_key"])
}
