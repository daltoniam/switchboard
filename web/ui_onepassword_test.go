package web

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

const uiOnePasswordTools = 9

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

// uiOnePasswordExecute runs one 1Password tool through the execute meta-tool.
func uiOnePasswordExecute(t *testing.T, app *uiApp, tool string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "onepassword-ui", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "execute", Arguments: map[string]any{"tool_name": tool, "arguments": args}})
	require.NoError(t, err)
	return result
}

// uiOnePasswordAddIdentity submits the add identity form on the setup page.
func uiOnePasswordAddIdentity(t *testing.T, page playwright.Page, id, label, account, token string) {
	t.Helper()
	form := page.Locator("#onepassword-identity-add")
	require.NoError(t, form.Locator("#identity_id").Fill(id))
	require.NoError(t, form.Locator("#identity_meta_label").Fill(label))
	require.NoError(t, form.Locator("#identity_cred_account_name").Fill(account))
	require.NoError(t, form.Locator("#identity_cred_service_account_token").Fill(token))
	_, err := page.ExpectNavigation(func() error {
		return form.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Save identity"}).Click()
	}, playwright.PageExpectNavigationOptions{URL: "**/integrations/onepassword/setup?*"})
	require.NoError(t, err)
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

	uiOnePasswordIdentities(t, app, page)
}

// uiOnePasswordIdentities adds named identities while the integration is
// disabled, so no desktop app sign-in or approval prompt runs, then enables
// it and lists them over MCP.
func uiOnePasswordIdentities(t *testing.T, app *uiApp, page playwright.Page) {
	t.Helper()
	page.OnDialog(func(dialog playwright.Dialog) { require.NoError(t, dialog.Accept()) })
	identities := page.Locator("#onepassword-identities")
	uiText(t, identities, "0 configured")
	uiText(t, identities.Locator("#onepassword-desktop-limit"), "Only one identity can sign in through the 1Password desktop app")

	uiOnePasswordAddIdentity(t, page, "work", "Work vaults", "", "ops_not-a-real-work-token")
	work := identities.Locator(`[data-onepassword-identity="work"]`)
	uiText(t, work, "Work vaults")
	uiText(t, work.Locator(".onepassword-identity-method"), "Service account token")
	require.NotContains(t, uiResultTextOf(t, page.Locator("body")), "ops_not-a-real-work-token", "identity tokens are never rendered")

	uiOnePasswordAddIdentity(t, page, "me", "", "Switchboard UI Test", "")
	if msg := page.Locator(".toast.flash-error"); uiCount(t, msg) > 0 {
		uiText(t, msg, "without CGO")
		t.Log("this Switchboard build cannot use the 1Password desktop app; skipping the second desktop identity case")
	} else {
		uiText(t, identities.Locator(`[data-onepassword-identity="me"] .onepassword-identity-method`), "Desktop app as Switchboard UI Test")
		uiText(t, identities.Locator("#onepassword-desktop-limit"), "The me identity uses the desktop app")
		uiOnePasswordAddIdentity(t, page, "family", "Family", "Family", "")
		rejection := page.Locator(".toast.flash-error")
		uiText(t, rejection, "only one identity can sign in through the 1Password desktop app")
		uiText(t, rejection, `identities "family" and "me"`)
		uiText(t, rejection, "service account token instead")
		require.Zero(t, uiCount(t, identities.Locator(`[data-onepassword-identity="family"]`)))
		uiCapture(t, page, "onepassword-identity-rejected")
		_, err := page.ExpectNavigation(func() error {
			return identities.Locator(`[data-onepassword-identity="me"] .identity-delete`).Click()
		}, playwright.PageExpectNavigationOptions{URL: "**/integrations/onepassword/setup?success=*"})
		require.NoError(t, err)
		require.Zero(t, uiCount(t, identities.Locator(`[data-onepassword-identity="me"]`)))
	}
	uiText(t, identities, "1 configured")

	require.NoError(t, page.Locator("#integration-configuration .toggle").Click())
	uiSave(t, page)
	uiCapture(t, page, "onepassword-identities-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "onepassword-identities-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))

	names, _ := uiOnePasswordMCP(t, app)
	require.Contains(t, names, "onepassword_list_identities")
	listed := uiResultText(t, uiOnePasswordExecute(t, app, "onepassword_list_identities", map[string]any{}))
	for _, want := range []string{`"work"`, "Work vaults", "service_account", `"default"`} {
		require.Contains(t, listed, want)
	}
	require.NotContains(t, listed, "ops_not-a-real")
	unknown := uiOnePasswordExecute(t, app, "onepassword_list_vaults", map[string]any{"identity_id": "nope"})
	require.True(t, unknown.IsError)
	require.Contains(t, uiResultText(t, unknown), `unknown identity "nope"`)
	routed := uiOnePasswordExecute(t, app, "onepassword_list_vaults", map[string]any{"identity_id": "work"})
	require.True(t, routed.IsError, "the fake work token cannot sign in")
	require.Contains(t, uiResultText(t, routed), "cannot sign in with the service account token")
	require.NotContains(t, uiResultText(t, routed), "ops_not-a-real-work-token")

	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/onepassword/setup")
	uiText(t, page.Locator(`#onepassword-identities [data-onepassword-identity="work"]`), "Work vaults")
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	ic := saved.Integrations["onepassword"]
	require.Equal(t, []string{"work"}, slices.Sorted(maps.Keys(ic.Identities)))
	require.Equal(t, "Work vaults", ic.Identities["work"].Metadata["label"])
	require.True(t, ic.Identities["work"].Credentials["service_account_token"] == "ops_not-a-real-work-token", "saved identity token differs from the one entered")
}

func uiCount(t *testing.T, locator playwright.Locator) int {
	t.Helper()
	n, err := locator.Count()
	require.NoError(t, err)
	return n
}

func uiResultTextOf(t *testing.T, locator playwright.Locator) string {
	t.Helper()
	text, err := locator.TextContent()
	require.NoError(t, err)
	return text
}
