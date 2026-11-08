package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/googleoauth"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestUIGoogleAdditionalAccount adds a second Google account through the
// unified setup page with a fixture authorize redirect and token endpoint,
// then removes it. No real Google sign-in happens.
func TestUIGoogleAdditionalAccount(t *testing.T) {
	_ = newUIApp(t)
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"work-fixture-secret","refresh_token":"work-fixture-refresh","scope":"https://mail.google.com/ https://www.googleapis.com/auth/gmail.settings.basic"}`))
	}))
	t.Cleanup(tokenSrv.Close)
	googleoauth.SetTokenURLForTest(t, tokenSrv.URL)
	googleoauth.Reset()

	_, reg, store := setupTestWeb()
	reg.Register(&mockIntegration{name: "gmail", healthy: true})
	seedGooglePrimary(store, "gmail")
	// The account delete handler only accepts same-origin requests to the
	// server's own port, so serve on the port the WebServer is built with.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ws := New(&mcp.Services{Config: store, Registry: reg}, listener.Addr().(*net.TCPAddr).Port, nil, nil)
	app := httptest.NewUnstartedServer(ws.Handler())
	app.Listener = listener
	app.Start()
	t.Cleanup(app.Close)

	page := uiPage(t)
	require.NoError(t, page.Route("https://accounts.google.com/**", func(route playwright.Route) {
		authorize, err := url.Parse(route.Request().URL())
		require.NoError(t, err)
		require.Equal(t, "consent select_account", authorize.Query().Get("prompt"))
		callback, err := url.Parse(app.URL + "/api/google/oauth/callback")
		require.NoError(t, err)
		callback.RawQuery = url.Values{"code": {"fixture-code"}, "state": {authorize.Query().Get("state")}}.Encode()
		require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(http.StatusFound), Headers: map[string]string{"Location": callback.String()}}))
	}))

	uiNavigate(t, page, app.URL+googleSetupPath)
	// Only Gmail has a main account, so connect the extra account there only.
	checks := page.Locator(".gsvc")
	count, err := checks.Count()
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		value, err := checks.Nth(i).GetAttribute("value")
		require.NoError(t, err)
		if value != "gmail" {
			require.NoError(t, checks.Nth(i).Uncheck())
		}
	}
	require.NoError(t, page.Locator("#google_account_id").Fill("work"))
	require.NoError(t, page.Locator("#google_account_label").Fill("Work fixture"))
	require.NoError(t, page.Locator("#account-start-btn").Click())
	uiText(t, page.Locator(`[data-google-account="work"]`), "Work fixture")

	content, err := page.Content()
	require.NoError(t, err)
	require.NotContains(t, content, "work-fixture-secret")
	require.NotContains(t, content, "primary-access")
	config, ok := store.GetIntegration("gmail")
	require.True(t, ok)
	require.Equal(t, "work-fixture-secret", config.Identities["work"].Credentials["access_token"])
	require.Equal(t, "primary-access", config.Credentials["access_token"])
	uiCapture(t, page, "google-additional-account")

	page.OnDialog(func(dialog playwright.Dialog) { require.NoError(t, dialog.Accept()) })
	require.NoError(t, page.Locator(`[data-google-account="work"] .identity-delete`).Click())
	uiText(t, page.Locator(".flash-success"), "Removed Google account work")
	config, ok = store.GetIntegration("gmail")
	require.True(t, ok)
	require.Empty(t, config.Identities)
	require.Equal(t, mcp.Credentials{
		mcp.CredKeyClientID: "cid", mcp.CredKeyClientSecret: "csec",
		"access_token": "primary-access", "refresh_token": "primary-refresh",
	}, config.Credentials)
}
