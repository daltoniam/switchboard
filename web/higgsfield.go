package web

import (
	"net/http"

	"github.com/daltoniam/switchboard/web/templates/pages"
)

func (w *WebServer) handleHiggsfieldSetup(rw http.ResponseWriter, r *http.Request) {
	ic, _ := w.services.Config.GetIntegration("higgsfield")
	data := pages.HiggsfieldSetupData{
		FlashResult: r.URL.Query().Get("result"),
		FlashError:  r.URL.Query().Get("error"),
	}
	if ic != nil {
		data.Enabled = ic.Enabled
		data.HasToken = ic.Credentials["mcp_access_token"] != ""
	}
	if entry, ok := w.health.get("higgsfield"); ok && entry.Enabled && !entry.CheckedAt.IsZero() {
		data.HealthChecked = true
		data.Healthy = entry.Healthy
	}
	page := w.pageData(r, "Higgsfield Setup", "/integrations")
	page.FlashError = ""
	pages.HiggsfieldSetup(page, data).Render(r.Context(), rw)
}
