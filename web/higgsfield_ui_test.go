package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/higgsfield"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// TestUIHiggsfield drives the Higgsfield setup page through a mocked OAuth
// provider; no real Higgsfield account is contacted.
func TestUIHiggsfield(t *testing.T) {
	if os.Getenv("SWITCHBOARD_UI_BINARY") == "" {
		t.Skip("run make test-ui")
	}
	for _, test := range []struct {
		name   string
		cancel bool
	}{
		{name: "connect"},
		{name: "cancel", cancel: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHiggsfieldOAuthFixture(t)
			ws, reg, store := setupTestWeb()
			integration := higgsfield.New()
			t.Cleanup(func() { require.NoError(t, integration.(io.Closer).Close()) })
			require.NoError(t, integration.Configure(t.Context(), mcp.Credentials{"base_url": fixture.server.URL, "mcp_access_token": "previous-fixture-token"}))
			reg.integrations["higgsfield"] = integration
			require.NoError(t, store.SetIntegration("higgsfield", &mcp.IntegrationConfig{Credentials: mcp.Credentials{"base_url": fixture.server.URL}}))
			app := httptest.NewServer(ws.Handler())
			t.Cleanup(app.Close)
			page := uiPage(t)
			require.NoError(t, page.Route(fixture.server.URL+"/oauth2/authorize*", func(route playwright.Route) {
				authorize, err := url.Parse(route.Request().URL())
				require.NoError(t, err)
				callback, err := url.Parse(app.URL + "/api/remote/higgsfield/oauth/callback")
				require.NoError(t, err)
				query := callback.Query()
				query.Set("state", authorize.Query().Get("state"))
				if test.cancel {
					query.Set("error", "access_denied")
				} else {
					query.Set("code", "fixture-code")
				}
				callback.RawQuery = query.Encode()
				require.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{Status: playwright.Int(http.StatusFound), Headers: map[string]string{"Location": callback.String()}}))
			}))

			uiNavigate(t, page, app.URL+"/integrations/higgsfield")
			uiText(t, page.Locator("h1"), "Higgsfield Setup")
			uiText(t, page.Locator(".badge").First(), "Not Connected")
			require.NoError(t, page.Locator("#higgsfield-oauth-start").Click())

			if test.cancel {
				uiText(t, page.Locator(".flash-error").First(), "access_denied")
				config, ok := store.GetIntegration("higgsfield")
				require.True(t, !ok || !config.Enabled)
				uiText(t, page.Locator(".badge").First(), "Not Connected")
				return
			}
			uiText(t, page.Locator(".flash-success"), "Connected via MCP OAuth")
			config, ok := store.GetIntegration("higgsfield")
			require.True(t, ok)
			require.True(t, config.Enabled)
			require.Equal(t, "higgsfield-oauth-token", config.Credentials["mcp_access_token"])
			content, err := page.Content()
			require.NoError(t, err)
			require.NotContains(t, content, "higgsfield-oauth-token")
			uiText(t, page.Locator("#higgsfield-oauth-start"), "Re-authorize with Higgsfield")

			result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			require.Contains(t, result.Data, `"credits":10`)

			uiCapture(t, page, "higgsfield-connected-desktop")
			require.NoError(t, page.SetViewportSize(390, 844))
			uiNavigate(t, page, app.URL+"/integrations/higgsfield/setup")
			uiCapture(t, page, "higgsfield-connected-mobile")
		})
	}
}
