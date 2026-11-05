package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	metabaseInt "github.com/daltoniam/switchboard/integrations/metabase"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

const metabaseProbeTimeout = 5 * time.Second

func (w *WebServer) handleMetabaseSetup(rw http.ResponseWriter, r *http.Request) {
	ic, _ := w.services.Config.GetIntegration("metabase")
	data := pages.MetabaseSetupData{
		FlashResult: r.URL.Query().Get("result"),
		FlashError:  r.URL.Query().Get("error"),
	}
	if ic != nil {
		data.Enabled = ic.Enabled
		data.URL = ic.Credentials["url"]
		data.HasAPIKey = ic.Credentials["api_key"] != ""
		data.HasOAuth = ic.Credentials["mcp_access_token"] != ""
		data.TokenSource = ic.Credentials[mcp.CredKeyTokenSource]
	}
	if entry, ok := w.health.get("metabase"); ok && entry.Enabled && !entry.CheckedAt.IsZero() {
		data.HealthChecked = true
		data.Healthy = entry.Healthy
	}
	if data.URL != "" {
		data.MCPProbe, data.MCPProbeError = probeMetabaseMCP(r.Context(), data.URL)
	}
	page := w.pageData(r, "Metabase Setup", "/integrations")
	page.FlashError = ""
	pages.MetabaseSetup(page, data).Render(r.Context(), rw)
}

// probeMetabaseMCP reads the instance-wide "MCP server" admin toggle so the page can
// offer OAuth only where it can succeed.
func probeMetabaseMCP(ctx context.Context, baseURL string) (state, detail string) {
	ctx, cancel := context.WithTimeout(ctx, metabaseProbeTimeout)
	defer cancel()
	enabled, err := metabaseInt.MCPServerEnabled(ctx, &http.Client{Timeout: metabaseProbeTimeout}, baseURL)
	switch {
	case err != nil:
		return pages.MetabaseMCPUnknown, err.Error()
	case enabled:
		return pages.MetabaseMCPEnabled, ""
	default:
		return pages.MetabaseMCPDisabled, ""
	}
}

func (w *WebServer) handleMetabaseSaveCredentials(rw http.ResponseWriter, r *http.Request) {
	redirect := func(key, msg string) {
		http.Redirect(rw, r, "/integrations/metabase/setup?"+key+"="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if err := r.ParseForm(); err != nil {
		redirect("error", "Invalid form data")
		return
	}
	baseURL := strings.TrimRight(strings.TrimSpace(r.FormValue("url")), "/")
	if baseURL == "" {
		redirect("error", "Metabase URL is required")
		return
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		redirect("error", "Metabase URL must start with http or https")
		return
	}
	apiKey := strings.TrimSpace(r.FormValue("api_key"))

	w.configMu.Lock()
	previous, _ := w.services.Config.GetIntegration("metabase")
	next := cloneIntegrationConfig(previous)
	result := "Metabase URL saved"
	// OAuth tokens belong to the host that issued them; a new URL invalidates them.
	if next.Credentials["url"] != baseURL && next.Credentials["mcp_access_token"] != "" {
		for _, key := range []string{"mcp_access_token", "mcp_refresh_token", "mcp_client_id"} {
			next.Credentials[key] = ""
		}
		next.Credentials[mcp.CredKeyTokenSource] = ""
		result = "Metabase URL saved; sign in again to use OAuth with the new instance"
	}
	next.Credentials["url"] = baseURL
	if apiKey != "" {
		next.Credentials["api_key"] = apiKey
		next.Enabled = true
		result = "API key saved"
	}
	if apiKey != "" || (next.Credentials["api_key"] != "" && next.Credentials["mcp_access_token"] == "") {
		next.Credentials[mcp.CredKeyTokenSource] = "api_key"
	}
	err := w.services.Config.SetIntegration("metabase", next)
	if err == nil {
		w.health.mu.Lock()
		delete(w.health.entries, "metabase")
		w.health.mu.Unlock()
		configurable := next.Credentials["api_key"] != "" || next.Credentials["mcp_access_token"] != ""
		if integration, ok := w.services.Registry.Get("metabase"); ok && configurable {
			err = mcp.ConfigureIntegration(r.Context(), integration, next)
		}
	}
	w.configMu.Unlock()
	if err != nil {
		redirect("error", err.Error())
		return
	}
	w.notifyConfigChanged()
	redirect("result", result)
}
