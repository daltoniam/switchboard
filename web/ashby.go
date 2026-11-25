package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

const ashbySetupPath = "/integrations/ashby/setup"

// ashbySignInKeys are the hosted MCP sign-in credentials. Saving an API key
// clears them so exactly one Ashby mode stays configured.
var ashbySignInKeys = []string{
	"mcp_access_token",
	remotemcp.CredKeyRefreshToken,
	remotemcp.CredKeyTokenExpiry,
	remotemcp.CredKeyTokenIssued,
	remotemcp.CredKeyClientID,
	remotemcp.CredKeyClientSecret,
}

func (w *WebServer) handleAshbySetup(rw http.ResponseWriter, r *http.Request) {
	ic, _ := w.services.Config.GetIntegration("ashby")
	data := pages.AshbySetupData{
		FlashResult: r.URL.Query().Get("result"),
		FlashError:  r.URL.Query().Get("error"),
	}
	if ic != nil {
		data.Enabled = ic.Enabled
		data.HasToken = ic.Credentials["mcp_access_token"] != ""
		data.HasAPIKey = strings.TrimSpace(ic.Credentials["api_key"]) != ""
	}
	if entry, ok := w.health.get("ashby"); ok && entry.Enabled && !entry.CheckedAt.IsZero() {
		data.HealthChecked = true
		data.Healthy = entry.Healthy
	}
	page := w.pageData(r, "Ashby Setup", "/integrations")
	page.FlashError = ""
	pages.AshbySetup(page, data).Render(r.Context(), rw)
}

// handleAshbyAPIKeySave switches Ashby to API key mode. A blank key keeps
// the saved one, so the enabled toggle can change without re-entering it.
func (w *WebServer) handleAshbyAPIKeySave(rw http.ResponseWriter, r *http.Request) {
	if !sameOriginForm(r) {
		http.Error(rw, "Same-origin request required", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, ashbySetupPath+"?error=Invalid+form+data", http.StatusSeeOther)
		return
	}
	key := strings.TrimSpace(r.FormValue("cred_api_key"))
	enabled := r.FormValue("enabled") == "true"

	w.configMu.Lock()
	previous, _ := w.services.Config.GetIntegration("ashby")
	next := cloneIntegrationConfig(previous)
	if key == "" {
		key = strings.TrimSpace(next.Credentials["api_key"])
	}
	if key == "" {
		w.configMu.Unlock()
		http.Redirect(rw, r, ashbySetupPath+"?error=API+key+is+required", http.StatusSeeOther)
		return
	}
	next.Enabled = enabled
	next.Credentials["api_key"] = key
	for _, signIn := range ashbySignInKeys {
		next.Credentials[signIn] = ""
	}
	next.Credentials[mcp.CredKeyTokenSource] = "api_key"
	err := w.services.Config.SetIntegration("ashby", next)
	w.configMu.Unlock()
	if err != nil {
		http.Redirect(rw, r, ashbySetupPath+"?error="+url.QueryEscape("Failed to save: "+err.Error()), http.StatusSeeOther)
		return
	}

	result := "Configuration saved"
	if integration, ok := w.services.Registry.Get("ashby"); ok {
		entry := healthEntry{Enabled: enabled, CheckedAt: time.Now()}
		if enabled {
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			if err := mcp.ConfigureIntegration(ctx, integration, next); err != nil {
				result = "Configuration saved, but the API key was rejected: " + err.Error()
			} else {
				entry.Healthy = integration.Healthy(ctx)
			}
			cancel()
		}
		w.health.mu.Lock()
		w.health.entries["ashby"] = entry
		w.health.mu.Unlock()
	}
	w.notifyConfigChanged()
	http.Redirect(rw, r, ashbySetupPath+"?result="+url.QueryEscape(result), http.StatusSeeOther)
}

// sameOriginForm accepts only form posts that prove they came from this
// page: Fetch Metadata, or failing that a matching Origin or Referer. A post
// with none of them is refused, because a cross-site page can suppress the
// Referer and some browsers send neither of the others.
func sameOriginForm(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		return matchesRequestOrigin(r, origin)
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		return matchesRequestOrigin(r, referer)
	}
	return false
}

// matchesRequestOrigin compares scheme and host with the origin this request
// was served from. A cross-site page cannot set X-Forwarded-Proto on a form
// post, so trusting it for TLS terminated by a proxy is safe here.
func matchesRequestOrigin(r *http.Request, raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return strings.EqualFold(parsed.Scheme, scheme) && strings.EqualFold(parsed.Host, r.Host)
}
