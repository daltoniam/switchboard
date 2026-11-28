package web

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strings"
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
		data.Identities, data.DesktopIdentity = onePasswordIdentities(ic)
		if ic.Enabled {
			data.Healthy, data.SignInError = onePasswordHealth(r.Context(), integration, ic)
		}
	}
	for _, t := range integration.Tools() {
		data.Tools = append(data.Tools, pages.ToolInfo{Name: string(t.Name), Description: t.Description})
	}
	page := w.pageData(r, "1Password Setup", "/integrations")
	pages.OnePasswordSetup(page, data).Render(r.Context(), rw)
}

// onePasswordIdentities lists the named identities without their tokens, and
// names the identity that signs in through the desktop app.
func onePasswordIdentities(ic *mcp.IntegrationConfig) ([]pages.OnePasswordIdentity, string) {
	desktop := ""
	if strings.TrimSpace(ic.Credentials["account_name"]) != "" {
		desktop = "default"
	}
	ids := slices.Sorted(maps.Keys(ic.Identities))
	out := make([]pages.OnePasswordIdentity, 0, len(ids))
	for _, id := range ids {
		identity := ic.Identities[id]
		account := strings.TrimSpace(identity.Credentials["account_name"])
		if account != "" && desktop == "" {
			desktop = id
		}
		out = append(out, pages.OnePasswordIdentity{ID: id, Label: identity.Metadata["label"], Account: account})
	}
	return out, desktop
}

// onePasswordHealth applies the saved credentials (keeping an existing
// session) and checks them, returning why sign-in failed when it did.
func onePasswordHealth(ctx context.Context, integration mcp.Integration, ic *mcp.IntegrationConfig) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, onePasswordHealthTimeout)
	defer cancel()
	if err := mcp.ConfigureIntegration(ctx, integration, ic); err != nil {
		return false, err.Error()
	}
	if integration.Healthy(ctx) {
		return true, ""
	}
	if reporter, ok := integration.(interface{ SignInError() string }); ok {
		return false, reporter.SignInError()
	}
	return false, ""
}
