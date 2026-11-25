package web

import (
	"net/http"

	"github.com/daltoniam/switchboard/web/templates/pages"
)

func (w *WebServer) handleAshbySetup(rw http.ResponseWriter, r *http.Request) {
	ic, _ := w.services.Config.GetIntegration("ashby")
	data := pages.AshbySetupData{
		FlashResult: r.URL.Query().Get("result"),
		FlashError:  r.URL.Query().Get("error"),
	}
	if ic != nil {
		data.Enabled = ic.Enabled
		data.HasToken = ic.Credentials["mcp_access_token"] != ""
	}
	if entry, ok := w.health.get("ashby"); ok && entry.Enabled && !entry.CheckedAt.IsZero() {
		data.HealthChecked = true
		data.Healthy = entry.Healthy
	}
	page := w.pageData(r, "Ashby Setup", "/integrations")
	page.FlashError = ""
	pages.AshbySetup(page, data).Render(r.Context(), rw)
}
