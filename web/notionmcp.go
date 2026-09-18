package web

import (
	"net/http"

	"github.com/daltoniam/switchboard/web/templates/pages"
)

func (w *WebServer) handleNotionMCPSetup(rw http.ResponseWriter, r *http.Request) {
	ic, _ := w.services.Config.GetIntegration("notion-mcp")
	data := pages.NotionMCPSetupData{
		FlashResult: r.URL.Query().Get("result"),
		FlashError:  r.URL.Query().Get("error"),
	}
	if ic != nil {
		data.Enabled = ic.Enabled
		data.HasToken = ic.Credentials["mcp_access_token"] != ""
	}
	if entry, ok := w.health.get("notion-mcp"); ok && entry.Enabled && !entry.CheckedAt.IsZero() {
		data.HealthChecked = true
		data.Healthy = entry.Healthy
	}
	page := w.pageData(r, "Notion MCP Setup", "/integrations")
	page.FlashError = ""
	pages.NotionMCPSetup(page, data).Render(r.Context(), rw)
}
