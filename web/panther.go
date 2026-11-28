package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/panther"
	"github.com/daltoniam/switchboard/remotemcp"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

const pantherSetupPath = "/integrations/panther/setup"

// pantherSignInKeys are the Remote MCP sign-in credentials. Saving an API
// token clears them so exactly one Panther mode stays configured.
var pantherSignInKeys = []string{
	remotemcp.CredKeyAccessToken,
	remotemcp.CredKeyRefreshToken,
	remotemcp.CredKeyTokenExpiry,
	remotemcp.CredKeyTokenIssued,
	remotemcp.CredKeyClientID,
	remotemcp.CredKeyClientSecret,
}

func (w *WebServer) handlePantherSetup(rw http.ResponseWriter, r *http.Request) {
	ic, _ := w.services.Config.GetIntegration("panther")
	data := pages.PantherSetupData{
		FlashResult: r.URL.Query().Get("result"),
		FlashError:  r.URL.Query().Get("error"),
	}
	if ic != nil {
		data.Enabled = ic.Enabled
		data.InstanceURL = strings.TrimSpace(ic.Credentials["instance_url"])
		data.HasToken = ic.Credentials[remotemcp.CredKeyAccessToken] != ""
		data.HasAPIToken = strings.TrimSpace(ic.Credentials["api_token"]) != ""
	}
	if entry, ok := w.health.get("panther"); ok && entry.Enabled && !entry.CheckedAt.IsZero() {
		data.HealthChecked = true
		data.Healthy = entry.Healthy
	}
	page := w.pageData(r, "Panther Setup", "/integrations")
	page.FlashError = ""
	pages.PantherSetup(page, data).Render(r.Context(), rw)
}

func pantherRedirect(rw http.ResponseWriter, r *http.Request, key, message string) {
	http.Redirect(rw, r, pantherSetupPath+"?"+key+"="+url.QueryEscape(message), http.StatusSeeOther)
}

// handlePantherInstanceSave saves the instance URL both modes use. A new
// instance clears both modes' credentials, which belong to the old one.
func (w *WebServer) handlePantherInstanceSave(rw http.ResponseWriter, r *http.Request) {
	if !sameOriginForm(r) {
		http.Error(rw, "Same-origin request required", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		pantherRedirect(rw, r, "error", "Invalid form data")
		return
	}
	instance, err := panther.NormalizeInstanceURL(r.FormValue("cred_instance_url"))
	if err != nil {
		pantherRedirect(rw, r, "error", strings.TrimPrefix(err.Error(), "panther: "))
		return
	}

	w.configMu.Lock()
	previous, _ := w.services.Config.GetIntegration("panther")
	next := cloneIntegrationConfig(previous)
	changed := strings.TrimSpace(next.Credentials["instance_url"]) != instance
	next.Credentials["instance_url"] = instance
	if changed {
		next.Credentials["api_token"] = ""
		for _, key := range pantherSignInKeys {
			next.Credentials[key] = ""
		}
		next.Credentials[mcp.CredKeyTokenSource] = ""
	}
	if err := w.services.Config.SetIntegration("panther", next); err != nil {
		w.configMu.Unlock()
		pantherRedirect(rw, r, "error", "Failed to save: "+err.Error())
		return
	}
	if changed {
		// The adapter still holds the old instance's session or token; it
		// is released under the lock so a concurrent save cannot interleave.
		if integration, ok := w.services.Registry.Get("panther"); ok {
			if closer, ok := integration.(io.Closer); ok {
				_ = closer.Close()
			}
		}
	}
	w.configMu.Unlock()
	if changed {
		w.health.mu.Lock()
		delete(w.health.entries, "panther")
		w.health.mu.Unlock()
		w.notifyConfigChanged()
		pantherRedirect(rw, r, "result", "Instance URL saved. Sign in with Panther or save an API token to connect.")
		return
	}
	pantherRedirect(rw, r, "result", "Instance URL unchanged")
}

// handlePantherAPITokenSave switches Panther to API token mode. A blank
// token keeps the saved one, so the enabled toggle can change without
// re-entering it.
func (w *WebServer) handlePantherAPITokenSave(rw http.ResponseWriter, r *http.Request) {
	if !sameOriginForm(r) {
		http.Error(rw, "Same-origin request required", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		pantherRedirect(rw, r, "error", "Invalid form data")
		return
	}
	token := strings.TrimSpace(r.FormValue("cred_api_token"))
	enabled := r.FormValue("enabled") == "true"
	instance := ""
	if raw := strings.TrimSpace(r.FormValue("cred_instance_url")); raw != "" {
		normalized, err := panther.NormalizeInstanceURL(raw)
		if err != nil {
			pantherRedirect(rw, r, "error", strings.TrimPrefix(err.Error(), "panther: "))
			return
		}
		instance = normalized
	}

	w.configMu.Lock()
	previous, _ := w.services.Config.GetIntegration("panther")
	next := cloneIntegrationConfig(previous)
	saved := strings.TrimSpace(next.Credentials["instance_url"])
	if instance == "" {
		instance = saved
	}
	if instance == "" {
		w.configMu.Unlock()
		pantherRedirect(rw, r, "error", "Save your Panther instance URL first")
		return
	}
	// The instance field is part of this form, so a new instance and its
	// token save together; a token kept from the old instance would not work.
	if instance != saved {
		next.Credentials["api_token"] = ""
	}
	next.Credentials["instance_url"] = instance
	if token == "" {
		token = strings.TrimSpace(next.Credentials["api_token"])
	}
	if token == "" {
		w.configMu.Unlock()
		pantherRedirect(rw, r, "error", "API token is required")
		return
	}
	next.Enabled = enabled
	next.Credentials["api_token"] = token
	for _, key := range pantherSignInKeys {
		next.Credentials[key] = ""
	}
	next.Credentials[mcp.CredKeyTokenSource] = "api_token"
	if err := w.services.Config.SetIntegration("panther", next); err != nil {
		w.configMu.Unlock()
		pantherRedirect(rw, r, "error", "Failed to save: "+err.Error())
		return
	}
	// The adapter is configured before the lock is released, so concurrent
	// saves reach it in the order they were persisted. API token mode does
	// no network I/O here; the health check runs after unlocking.
	result := "Configuration saved"
	integration, registered := w.services.Registry.Get("panther")
	configured := false
	if registered && enabled {
		if err := mcp.ConfigureIntegration(r.Context(), integration, next); err != nil {
			result = "Configuration saved, but the API token was rejected: " + err.Error()
		} else {
			configured = true
		}
	}
	w.configMu.Unlock()

	if registered {
		entry := healthEntry{Enabled: enabled, CheckedAt: time.Now()}
		if configured {
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			if entry.Healthy = integration.Healthy(ctx); !entry.Healthy {
				result = "Configuration saved, but Panther did not accept the API token. Check the instance URL and the token under Settings → Developer Tools → API Tokens."
			}
			cancel()
		}
		w.health.mu.Lock()
		w.health.entries["panther"] = entry
		w.health.mu.Unlock()
	}
	w.notifyConfigChanged()
	pantherRedirect(rw, r, "result", result)
}

// handlePantherOAuthStart begins Sign in with Panther against the saved
// instance's Remote MCP server. It reads the instance from the saved
// configuration, because a disabled or never-configured adapter has none.
func (w *WebServer) handlePantherOAuthStart(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	reply := func(key, value string) { _ = json.NewEncoder(rw).Encode(map[string]string{key: value}) }
	if !sameOriginForm(r) {
		rw.WriteHeader(http.StatusForbidden)
		reply("error", "Same-origin request required")
		return
	}
	ic, _ := w.services.Config.GetIntegration("panther")
	instance := ""
	if ic != nil {
		instance = ic.Credentials["instance_url"]
	}
	if strings.TrimSpace(instance) == "" {
		reply("error", "Save your Panther instance URL first")
		return
	}
	origin, resource, err := panther.SignInTarget(instance)
	if err != nil {
		reply("error", err.Error())
		return
	}
	profile := remoteOAuthProfiles["panther"]
	profile.Options.Resource = resource
	redirectURI := fmt.Sprintf("http://localhost:%d%s", w.port, profile.CallbackPath)
	authorizeURL, err := remotemcp.StartOAuth("panther", origin, redirectURI, profile.Options)
	if err != nil {
		reply("error", "Could not start Sign in with Panther at "+origin+": "+err.Error()+". Remote MCP needs Panther 1.124 or later and an api. subdomain; otherwise use an API token.")
		return
	}
	reply("authorize_url", authorizeURL)
}
