package web

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daltoniam/switchboard/pluginoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginOAuthCredentialMutationRequiresLocalOrigin(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, mode := range []string{"origin", "host", "remote", "fetch-site", "same-site"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				w, _, cfg := pluginOAuthWeb(t)
				before := maps.Clone(cfg.cfg.Integrations["primer"].Credentials)
				path, body := "/integrations/primer", "cred_oauth_issuer=https%3A%2F%2Fevil.example"
				if method == http.MethodPut {
					path, body = "/api/integrations/primer/credentials", `{"oauth_issuer":"https://evil.example"}`
				}
				r := localOAuthRequest(method, path, body)
				if method == http.MethodPost {
					r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				}
				switch mode {
				case "origin":
					r.Header.Set("Origin", "https://evil.example")
				case "host":
					r.Host = "evil.example:3847"
				case "remote":
					r.RemoteAddr = "192.0.2.1:54321"
				case "fetch-site":
					r.Header.Set("Sec-Fetch-Site", "cross-site")
				case "same-site":
					r.Header.Set("Sec-Fetch-Site", "same-site")
				}
				rr := httptest.NewRecorder()
				w.Handler().ServeHTTP(rr, r)
				assert.Equal(t, http.StatusForbidden, rr.Code)
				assert.Equal(t, before, cfg.cfg.Integrations["primer"].Credentials)
			})
		}
	}
}

func TestPluginOAuthSettingsEditsClearTokens(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			w, _, cfg := pluginOAuthWeb(t)
			creds := cfg.cfg.Integrations["primer"].Credentials
			creds["oauth_access_token"], creds["oauth_expires_at"] = "old-access", "2030-01-01T00:00:00Z"
			creds["oauth_token_key"], creds["tasks_api_key"] = "tasks_api_key", "old-guest-access"
			path, body := "/integrations/primer", "cred_oauth_issuer=https%3A%2F%2Fnew.example&cred_oauth_token_key=other_key"
			if method == http.MethodPut {
				path, body = "/api/integrations/primer/credentials", `{"oauth_issuer":"https://new.example","oauth_token_key":"other_key"}`
			}
			r := localOAuthRequest(method, path, body)
			if method == http.MethodPost {
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			rr := httptest.NewRecorder()
			w.Handler().ServeHTTP(rr, r)
			require.Less(t, rr.Code, 400)
			saved := cfg.cfg.Integrations["primer"].Credentials
			assert.Equal(t, "https://new.example", saved["oauth_issuer"])
			for _, key := range []string{"oauth_access_token", "oauth_refresh_token", "oauth_expires_at", "tasks_api_key", "other_key"} {
				assert.Empty(t, saved[key], key)
			}
		})
	}
}

func TestPluginOAuthPUTCannotSupplyManagedTokens(t *testing.T) {
	w, _, cfg := pluginOAuthWeb(t)
	rr := httptest.NewRecorder()
	w.Handler().ServeHTTP(rr, localOAuthRequest(http.MethodPut, "/api/integrations/primer/credentials", `{"oauth_refresh_token":"injected","oauth_access_token":"injected","base_url":"https://api.example"}`))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "native-only", cfg.cfg.Integrations["primer"].Credentials["oauth_refresh_token"])
	assert.Empty(t, cfg.cfg.Integrations["primer"].Credentials["oauth_access_token"])
}

func TestPluginOAuthInvalidStatePreservesBrowserCookie(t *testing.T) {
	w, i, _ := pluginOAuthWeb(t)
	i.err = pluginoauth.ErrState
	r := localOAuthRequest(http.MethodGet, "/api/integrations/primer/oauth/callback?code=code&state=other", "")
	r.AddCookie(&http.Cookie{Name: "switchboard_oauth", Value: "browser"})
	rr := httptest.NewRecorder()
	w.Handler().ServeHTTP(rr, r)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Empty(t, rr.Result().Cookies())
}
