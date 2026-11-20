package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const uiOnePasswordTools = 6

// uiOnePasswordMCP searches for 1Password tools over MCP and, when they are
// enabled, lists vaults through execute. It returns the tool names and the
// list_vaults result.
func uiOnePasswordMCP(t *testing.T, app *uiApp) ([]string, *mcpsdk.CallToolResult) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "onepassword-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: map[string]any{"integration": "onepassword", "query": "1password vault items", "limit": 50}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	names := uiToolNames(t, text.Text)
	if len(names) == 0 {
		return nil, nil
	}
	require.Contains(t, names, "onepassword_list_vaults")
	result, err = session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": "onepassword_list_vaults", "arguments": map[string]any{}}})
	require.NoError(t, err)
	return names, result
}

func uiResultText(t *testing.T, result *mcpsdk.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok)
	return text.Text
}

// TestUIOnePassword drives setup without a fixture provider: the 1Password
// SDK signs in through its own WebAssembly core and protocol, which no local
// stub can answer. A malformed token proves the failure path. Set
// OP_LIVE_SERVICE_ACCOUNT_TOKEN and OP_LIVE_VAULT to also prove a real
// sign-in and vault read.
func TestUIOnePassword(t *testing.T) {
	app := newUIApp(t)
	liveToken := os.Getenv("OP_LIVE_SERVICE_ACCOUNT_TOKEN")
	liveVault := os.Getenv("OP_LIVE_VAULT")
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/onepassword"]`)
	uiText(t, card, "Disabled")
	names, _ := uiOnePasswordMCP(t, app)
	require.Empty(t, names)
	require.NoError(t, card.Click())
	form := page.Locator("#integration-configuration")
	input := form.Locator("#cred_service_account_token")
	kind, err := input.GetAttribute("type")
	require.NoError(t, err)
	require.Equal(t, "password", kind)
	value, err := input.InputValue()
	require.NoError(t, err)
	require.Empty(t, value)
	placeholder, err := input.GetAttribute("placeholder")
	require.NoError(t, err)
	require.Contains(t, placeholder, "ops_")
	label, err := form.Locator(`label[for="cred_service_account_token"]`).TextContent()
	require.NoError(t, err)
	require.Equal(t, "service_account_token", label)
	uiText(t, page.Locator("body"), fmt.Sprintf("Available Tools (%d)", uiOnePasswordTools))

	require.NoError(t, input.Fill("ops_not-a-real-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	names, result := uiOnePasswordMCP(t, app)
	require.Len(t, names, uiOnePasswordTools)
	require.True(t, result.IsError)
	require.Contains(t, uiResultText(t, result), "cannot sign in")
	require.NotContains(t, uiResultText(t, result), "ops_not-a-real-token", "errors must not echo the token")

	wantToken, wantBadge := "ops_not-a-real-token", "Not Verified"
	if liveToken != "" && liveVault != "" {
		wantToken, wantBadge = liveToken, "Connected"
		require.NoError(t, input.Fill(liveToken))
		uiSave(t, page)
		uiText(t, page.Locator(".badge").First(), "Connected")
		names, result = uiOnePasswordMCP(t, app)
		require.Len(t, names, uiOnePasswordTools)
		require.False(t, result.IsError, uiResultText(t, result))
		require.Contains(t, uiResultText(t, result), liveVault)
	} else {
		t.Log("OP_LIVE_SERVICE_ACCOUNT_TOKEN and OP_LIVE_VAULT are not set; real sign-in not verified")
	}
	uiCapture(t, page, "onepassword-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "onepassword-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/onepassword")
	uiText(t, page.Locator(".badge").First(), wantBadge)
	names, result = uiOnePasswordMCP(t, app)
	require.Len(t, names, uiOnePasswordTools)
	require.NotNil(t, result, "execution must still run after a restart")
	if liveToken != "" && liveVault != "" {
		require.False(t, result.IsError, uiResultText(t, result))
		require.Contains(t, uiResultText(t, result), liveVault)
	} else {
		require.True(t, result.IsError)
		require.Contains(t, uiResultText(t, result), "cannot sign in")
		require.NotContains(t, uiResultText(t, result), "ops_not-a-real-token")
	}
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	names, _ = uiOnePasswordMCP(t, app)
	require.Empty(t, names)

	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.False(t, saved.Integrations["onepassword"].Enabled)
	// Compared as a bool so a failure never prints the token into CI logs.
	require.True(t, saved.Integrations["onepassword"].Credentials["service_account_token"] == wantToken, "saved service account token differs from the one entered")
}
