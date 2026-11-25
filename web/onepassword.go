package web

import (
	"context"
	"net/http"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/onepassword"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

// onePasswordHealthTimeout bounds the health check on page load. Desktop app
// sign-in waits while the user approves the 1Password prompt.
const onePasswordHealthTimeout = 60 * time.Second

func (w *WebServer) handleOnePasswordSetup(rw http.ResponseWriter, r *http.Request) {
	integration, ok := w.services.Registry.Get("onepassword")
	if !ok {
		http.NotFound(rw, r)
		return
	}
	data := pages.OnePasswordSetupData{DesktopAvailable: onepassword.DesktopAppAvailable()}
	if ic, exists := w.services.Config.GetIntegration("onepassword"); exists {
		data.Enabled = ic.Enabled
		data.AccountName = ic.Credentials["account_name"]
		data.ServiceAccountToken = ic.Credentials["service_account_token"]
		if ic.Enabled {
			ctx, cancel := context.WithTimeout(r.Context(), onePasswordHealthTimeout)
			if err := mcp.ConfigureIntegration(ctx, integration, ic); err != nil {
				data.SignInError = err.Error()
			} else {
				data.Healthy = integration.Healthy(ctx)
				if reporter, ok := integration.(interface{ SignInError() string }); ok && !data.Healthy {
					data.SignInError = reporter.SignInError()
				}
			}
			cancel()
		}
	}
	for _, t := range integration.Tools() {
		data.Tools = append(data.Tools, pages.ToolInfo{Name: string(t.Name), Description: t.Description})
	}
	page := w.pageData(r, "1Password Setup", "/integrations")
	pages.OnePasswordSetup(page, data).Render(r.Context(), rw)
}
