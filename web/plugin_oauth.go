package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/pluginoauth"
)

func oauthResponseHeaders(rw http.ResponseWriter) {
	rw.Header().Set("Cache-Control", "no-store")
	rw.Header().Set("Referrer-Policy", "no-referrer")
	rw.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
}

func (w *WebServer) localOAuthRequest(r *http.Request, start bool) bool {
	return w.localRequest(r, start, false)
}

func (w *WebServer) localRequest(r *http.Request, mutation, allowLocalhost bool) bool {
	host := fmt.Sprintf("127.0.0.1:%d", w.port)
	if allowLocalhost && r.Host == fmt.Sprintf("localhost:%d", w.port) {
		host = r.Host
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(peer).IsLoopback() || r.Host != host || w.port < 1 || w.port > 65535 {
		return false
	}
	if !mutation {
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+host {
		return false
	}
	site := r.Header.Get("Sec-Fetch-Site")
	return site == "" || site == "same-origin" || site == "none"
}

func (w *WebServer) oauthIntegration(name string) (mcp.OAuthIntegration, bool) {
	integration, ok := w.services.Registry.Get(name)
	if !ok {
		return nil, false
	}
	oauth, ok := integration.(mcp.OAuthIntegration)
	return oauth, ok
}

func (w *WebServer) handlePluginOAuthStart(rw http.ResponseWriter, r *http.Request) {
	oauthResponseHeaders(rw)
	if r.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		http.Error(rw, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !w.localOAuthRequest(r, true) {
		http.Error(rw, "Local same-origin request required", http.StatusForbidden)
		return
	}
	name := r.PathValue("name")
	oauth, ok := w.oauthIntegration(name)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	creds, err := w.pluginOAuthCredentials(rw, r, name)
	if err != nil {
		http.Error(rw, "Invalid OAuth configuration", http.StatusBadRequest)
		return
	}
	secret := make([]byte, 32)
	if _, err := io.ReadFull(w.oauthRandReader, secret); err != nil {
		http.Error(rw, "OAuth session creation failed; retry connecting", http.StatusInternalServerError)
		return
	}
	browser := base64.RawURLEncoding.EncodeToString(secret)
	callback := "/api/integrations/" + name + "/oauth/callback"
	redirect := fmt.Sprintf("http://127.0.0.1:%d%s", w.port, callback)
	authorizeURL, err := oauth.StartOAuth(r.Context(), creds, redirect, browser)
	if err != nil {
		http.Error(rw, "OAuth start failed; check configuration and persistence", http.StatusBadRequest)
		return
	}
	http.SetCookie(rw, &http.Cookie{Name: "switchboard_oauth", Value: browser, Path: callback, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	rw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(rw).Encode(map[string]string{"authorize_url": authorizeURL}); err != nil {
		return
	}
}

func (w *WebServer) pluginOAuthCredentials(rw http.ResponseWriter, r *http.Request, name string) (mcp.Credentials, error) {
	var body struct {
		Credentials mcp.Credentials `json:"credentials"`
	}
	if r.ContentLength != 0 {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			return nil, errors.New("invalid content type")
		}
		decoder := json.NewDecoder(http.MaxBytesReader(rw, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			return nil, errors.New("invalid body")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return nil, errors.New("trailing body")
		}
	}
	ic, ok := w.services.Config.GetIntegration(name)
	if !ok || ic == nil {
		return nil, errors.New("missing integration configuration")
	}
	creds := maps.Clone(ic.Credentials)
	if creds == nil {
		creds = mcp.Credentials{}
	}
	for key, value := range body.Credentials {
		switch key {
		case "oauth_access_token", "oauth_refresh_token", "oauth_expires_at", "oauth_client_secret":
			return nil, errors.New("managed credential")
		}
		if strings.HasPrefix(key, "oauth_") && key != "oauth_issuer" && key != "oauth_client_id" && key != "oauth_scopes" && key != "oauth_token_key" && key != "oauth_subject" && key != "oauth_email" {
			return nil, errors.New("unknown OAuth credential")
		}
		creds[key] = value
	}
	return creds, nil
}

func (w *WebServer) handlePluginOAuthCallback(rw http.ResponseWriter, r *http.Request) {
	oauthResponseHeaders(rw)
	if !w.localOAuthRequest(r, false) {
		http.Error(rw, "Local request required", http.StatusForbidden)
		return
	}
	name := r.PathValue("name")
	oauth, ok := w.oauthIntegration(name)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	cookie, err := r.Cookie("switchboard_oauth")
	query := r.URL.Query()
	if err != nil || cookie.Value == "" || len(query["state"]) != 1 || query.Get("state") == "" || len(query["code"]) > 1 || len(query["iss"]) > 1 || len(r.URL.RawQuery) > 16<<10 {
		http.Error(rw, "Invalid OAuth callback; connect again", http.StatusBadRequest)
		return
	}
	code := query.Get("code")
	if query.Has("error") {
		code = ""
	}
	if err := oauth.CompleteOAuth(r.Context(), code, query.Get("state"), cookie.Value, query.Get("iss")); err != nil {
		http.Error(rw, "OAuth connection failed; retry the integration or connect again", http.StatusBadRequest)
		return
	}
	http.SetCookie(rw, &http.Cookie{Name: "switchboard_oauth", Path: "/api/integrations/" + name + "/oauth/callback", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.notifyConfigChanged()
	http.Redirect(rw, r, "/integrations/"+name, http.StatusSeeOther)
}

func managedOAuthCredential(key string, creds mcp.Credentials) bool {
	return pluginoauth.ManagedCredential(key, creds)
}

func (w *WebServer) editPluginCredentials(r *http.Request, name string, updates mcp.Credentials, enabled bool) error {
	integration, _ := w.services.Registry.Get(name)
	if editor, ok := integration.(mcp.CredentialEditor); ok {
		return editor.EditCredentials(r.Context(), updates, enabled)
	}
	update := func(ic *mcp.IntegrationConfig) error {
		ic.Credentials = pluginoauth.MergeCredentials(ic.Credentials, updates)
		ic.Enabled = enabled
		return nil
	}
	if atomic, ok := w.services.Config.(mcp.IntegrationConfigUpdater); ok {
		return atomic.UpdateIntegration(name, update)
	}
	existing, _ := w.services.Config.GetIntegration(name)
	ic := cloneIntegrationConfig(existing)
	if err := update(ic); err != nil {
		return err
	}
	return w.services.Config.SetIntegration(name, ic)
}
