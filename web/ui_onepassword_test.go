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
	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

const uiOnePasswordTools = 8

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

// uiOnePasswordAppInstalled reports whether the host has the 1Password app's
// SDK library, so the test never sends a real approval prompt to a developer.
func uiOnePasswordAppInstalled() bool {
	home, _ := os.UserHomeDir()
	for _, path := range []string{
		"/Applications/1Password.app/Contents/Frameworks/libop_sdk_ipc_client.dylib",
		filepath.Join(home, "Applications/1Password.app/Contents/Frameworks/libop_sdk_ipc_client.dylib"),
		"/usr/bin/1password/libop_sdk_ipc_client.so",
		"/opt/1Password/libop_sdk_ipc_client.so",
		"/snap/bin/1password/libop_sdk_ipc_client.so",
	} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func uiAttr(t *testing.T, locator playwright.Locator, name string) string {
	t.Helper()
	value, err := locator.GetAttribute(name)
	require.NoError(t, err)
	return value
}

func uiValue(t *testing.T, locator playwright.Locator) string {
	t.Helper()
	value, err := locator.InputValue()
	require.NoError(t, err)
	return value
}

// TestUIOnePassword drives the setup page without a fixture provider: the
// 1Password SDK signs in through its own core and protocol (or the desktop
// app), which no local stub can answer. Conflicting credentials, an
// unreachable desktop app and a malformed token prove the failure paths. Set
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
	require.NoError(t, page.WaitForURL("**/integrations/onepassword/setup"))
	uiText(t, page.Locator(".badge").First(), "Not Configured")

	form := page.Locator("#integration-configuration")
	account := form.Locator("#cred_account_name")
	token := form.Locator("#cred_service_account_token")
	require.Equal(t, "text", uiAttr(t, account, "type"), "the account name is not a secret")
	require.Equal(t, "password", uiAttr(t, token, "type"))
	require.Empty(t, uiValue(t, account))
	require.Empty(t, uiValue(t, token))
	require.Contains(t, uiAttr(t, account, "placeholder"), "desktop app sidebar")
	require.Contains(t, uiAttr(t, token, "placeholder"), "ops_")
	uiText(t, form.Locator(`label[for="cred_account_name"]`), "Account name")
	uiText(t, form.Locator(`label[for="cred_service_account_token"]`), "Service account token")
	uiText(t, form, "Integrate with 1Password SDKs")
	uiText(t, form, "Developer Integrations")
	wizard := form.Locator(`a[href^="https://start.1password.com/developer-tools/infrastructure-secrets/serviceaccount/"]`)
	uiText(t, wizard, "service account wizard")
	require.Equal(t, "noopener noreferrer", uiAttr(t, wizard, "rel"))
	uiText(t, page.Locator("body"), fmt.Sprintf("Available Tools (%d)", uiOnePasswordTools))

	signInError := page.Locator("#onepassword-sign-in-error")
	require.NoError(t, account.Fill("Switchboard UI Test"))
	require.NoError(t, token.Fill("ops_not-a-real-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	uiText(t, signInError, "not both")
	require.NotContains(t, uiResultTextOf(t, signInError), "ops_not-a-real-token", "errors must not echo the token")

	if uiOnePasswordAppInstalled() {
		t.Log("the 1Password app is installed on this host; skipping the unreachable desktop app case so no real approval prompt appears")
	} else {
		require.NoError(t, token.Fill(""))
		uiSave(t, page)
		uiText(t, page.Locator(".badge").First(), "Not Verified")
		uiText(t, page.Locator("#onepassword-method"), "desktop app as Switchboard UI Test")
		uiText(t, signInError, "cannot sign in through the 1Password desktop app")
		uiText(t, signInError, "Integrate with 1Password SDKs")
		names, result := uiOnePasswordMCP(t, app)
		require.Len(t, names, uiOnePasswordTools)
		require.True(t, result.IsError)
		require.Contains(t, uiResultText(t, result), "cannot sign in through the 1Password desktop app")
		require.Contains(t, uiResultText(t, result), "Docker")
	}

	require.NoError(t, account.Fill(""))
	require.NoError(t, token.Fill("ops_not-a-real-token"))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	uiText(t, page.Locator("#onepassword-method"), "service account token")
	uiText(t, signInError, "cannot sign in with the service account token")
	names, result := uiOnePasswordMCP(t, app)
	require.Len(t, names, uiOnePasswordTools)
	require.True(t, result.IsError)
	require.Contains(t, uiResultText(t, result), "cannot sign in")
	require.NotContains(t, uiResultText(t, result), "ops_not-a-real-token", "errors must not echo the token")

	wantToken, wantBadge := "ops_not-a-real-token", "Not Verified"
	if liveToken != "" && liveVault != "" {
		wantToken, wantBadge = liveToken, "Connected"
		require.NoError(t, token.Fill(liveToken))
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
	require.NoError(t, page.WaitForURL("**/integrations/onepassword/setup"))
	uiText(t, page.Locator(".badge").First(), wantBadge)
	require.Empty(t, uiValue(t, account))
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
	require.Empty(t, saved.Integrations["onepassword"].Credentials["account_name"])
	// Compared as a bool so a failure never prints the token into CI logs.
	require.True(t, saved.Integrations["onepassword"].Credentials["service_account_token"] == wantToken, "saved service account token differs from the one entered")
}

func uiResultTextOf(t *testing.T, locator playwright.Locator) string {
	t.Helper()
	text, err := locator.TextContent()
	require.NoError(t, err)
	return text
}
