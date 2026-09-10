package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type oauthIntegration struct {
	mockIntegration
	redirect string
	browser  string
	code     string
	state    string
	issuer   string
	calls    int
	err      error
}

func (i *oauthIntegration) StartOAuth(_ context.Context, creds mcp.Credentials, redirect, browser string) (string, error) {
	i.lastCreds, i.redirect, i.browser = creds, redirect, browser
	i.calls++
	return "https://issuer.example/authorize?state=random", i.err
}

func (i *oauthIntegration) CompleteOAuth(_ context.Context, code, state, browser, issuer string) error {
	i.code, i.state, i.browser, i.issuer = code, state, browser, issuer
	i.calls++
	return i.err
}

func pluginOAuthWeb(t *testing.T) (*WebServer, *oauthIntegration, *mockConfigService) {
	t.Helper()
	w, reg, cfg := setupTestWeb()
	i := &oauthIntegration{mockIntegration: mockIntegration{name: "primer"}}
	require.NoError(t, reg.Register(i))
	cfg.cfg.Integrations["primer"] = &mcp.IntegrationConfig{Credentials: mcp.Credentials{"oauth_issuer": "https://issuer.example", "oauth_client_id": "saved-client", "oauth_refresh_token": "native-only"}, ToolGlobs: []string{"primer_read_*"}}
	return w, i, cfg
}

func localOAuthRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, "http://127.0.0.1:3847"+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:54321"
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func TestPluginOAuthStart(t *testing.T) {
	for _, body := range []string{"", "{}", `{"credentials":{"oauth_client_id":"new-client"}}`} {
		t.Run(body, func(t *testing.T) {
			w, i, cfg := pluginOAuthWeb(t)
			r := localOAuthRequest(http.MethodPost, "/api/integrations/primer/oauth/start", body)
			r.Header.Set("Origin", "http://127.0.0.1:3847")
			r.Header.Set("X-Forwarded-Host", "evil.example")
			rr := httptest.NewRecorder()
			w.Handler().ServeHTTP(rr, r)
			require.Equal(t, http.StatusOK, rr.Code)
			assert.JSONEq(t, `{"authorize_url":"https://issuer.example/authorize?state=random"}`, rr.Body.String())
			assert.Equal(t, "http://127.0.0.1:3847/api/integrations/primer/oauth/callback", i.redirect)
			assert.Equal(t, "https://issuer.example", i.lastCreds["oauth_issuer"])
			assert.Equal(t, "saved-client", cfg.cfg.Integrations["primer"].Credentials["oauth_client_id"])
			cookies := rr.Result().Cookies()
			require.Len(t, cookies, 1)
			cookie := cookies[0]
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			assert.Equal(t, "/api/integrations/primer/oauth/callback", cookie.Path)
			assert.Equal(t, 600, cookie.MaxAge)
			assert.Equal(t, i.browser, cookie.Value)
			assert.NotEmpty(t, cookie.Value)
			assert.NotContains(t, rr.Body.String(), "native-only")
			assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
		})
	}
}

func TestPluginOAuthStartRejectsUnsafeRequests(t *testing.T) {
	for _, name := range []string{"get", "origin", "null-origin", "host", "localhost", "remote", "fetch-site", "form", "oversize", "trailing-json", "token-input", "unknown-field"} {
		t.Run(name, func(t *testing.T) {
			w, i, _ := pluginOAuthWeb(t)
			r := localOAuthRequest(http.MethodPost, "/api/integrations/primer/oauth/start", "{}")
			switch name {
			case "get":
				r.Method = http.MethodGet
			case "origin":
				r.Header.Set("Origin", "https://evil.example")
			case "null-origin":
				r.Header.Set("Origin", "null")
			case "host":
				r.Host = "evil.example:3847"
			case "localhost":
				r.Host = "localhost:3847"
			case "remote":
				r.RemoteAddr = "192.0.2.1:4321"
			case "fetch-site":
				r.Header.Set("Sec-Fetch-Site", "cross-site")
			case "form":
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			case "oversize":
				r = localOAuthRequest(http.MethodPost, r.URL.Path, `{"credentials":{"x":"`+strings.Repeat("x", 65536)+`"}}`)
			case "trailing-json":
				r = localOAuthRequest(http.MethodPost, r.URL.Path, `{} {}`)
			case "token-input":
				r = localOAuthRequest(http.MethodPost, r.URL.Path, `{"credentials":{"oauth_refresh_token":"secret"}}`)
			case "unknown-field":
				r = localOAuthRequest(http.MethodPost, r.URL.Path, `{"enabled":true}`)
			}
			rr := httptest.NewRecorder()
			w.Handler().ServeHTTP(rr, r)
			assert.GreaterOrEqual(t, rr.Code, 400)
			assert.Zero(t, i.calls)
		})
	}
}

func TestPluginOAuthCallback(t *testing.T) {
	for _, name := range []string{"success", "provider-error", "missing-cookie", "missing-state", "duplicate-code", "denied"} {
		t.Run(name, func(t *testing.T) {
			w, i, _ := pluginOAuthWeb(t)
			if name == "provider-error" {
				i.err = errors.New("secret-token-error")
			}
			r := localOAuthRequest(http.MethodGet, "/api/integrations/primer/oauth/callback?code=secret-code&state=secret-state&iss=https%3A%2F%2Fissuer.example", "")
			if name != "missing-cookie" {
				r.AddCookie(&http.Cookie{Name: "switchboard_oauth", Value: "browser"})
			}
			if name == "missing-state" {
				r.URL.RawQuery = "code=secret-code"
			}
			if name == "duplicate-code" {
				r.URL.RawQuery += "&code=other"
			}
			if name == "denied" {
				r.URL.RawQuery += "&error=access_denied&error_description=secret-description"
			}
			rr := httptest.NewRecorder()
			w.Handler().ServeHTTP(rr, r)
			assert.NotContains(t, rr.Body.String()+rr.Header().Get("Location"), "secret")
			assert.Equal(t, "no-referrer", rr.Header().Get("Referrer-Policy"))
			if name == "success" {
				assert.Equal(t, http.StatusSeeOther, rr.Code)
				assert.Equal(t, "/integrations/primer", rr.Header().Get("Location"))
				assert.Equal(t, "secret-code", i.code)
				assert.Equal(t, "secret-state", i.state)
				assert.Equal(t, "https://issuer.example", i.issuer)
			} else if name == "denied" {
				assert.Empty(t, i.code)
			} else if name != "provider-error" {
				assert.Zero(t, i.calls)
			}
		})
	}
}

func TestPluginOAuthAvailability(t *testing.T) {
	for _, name := range []string{"missing", "testint"} {
		t.Run(name, func(t *testing.T) {
			w, _, _ := setupTestWeb()
			rr := httptest.NewRecorder()
			w.Handler().ServeHTTP(rr, localOAuthRequest(http.MethodPost, "/api/integrations/"+name+"/oauth/start", ""))
			assert.Equal(t, http.StatusNotFound, rr.Code)
		})
	}
}

func TestPluginOAuthDetailHidesTokensAndOffersConnect(t *testing.T) {
	w, _, cfg := pluginOAuthWeb(t)
	ic := cfg.cfg.Integrations["primer"]
	ic.Credentials["oauth_access_token"] = "access-secret"
	ic.Credentials["oauth_token_key"] = "tasks_api_key"
	ic.Credentials["tasks_api_key"] = "legacy-token-secret"
	rr := httptest.NewRecorder()
	w.Handler().ServeHTTP(rr, localOAuthRequest(http.MethodGet, "/integrations/primer", ""))
	require.Equal(t, http.StatusOK, rr.Code)
	for _, secret := range []string{"native-only", "access-secret", "legacy-token-secret"} {
		assert.NotContains(t, rr.Body.String(), secret)
	}
	assert.Contains(t, rr.Body.String(), "Connect OAuth")
}

func TestPluginOAuthFormSavePreservesNativeTokens(t *testing.T) {
	w, _, cfg := pluginOAuthWeb(t)
	r := localOAuthRequest(http.MethodPost, "/integrations/primer", "enabled=true&cred_oauth_issuer=https%3A%2F%2Fissuer.example&cred_oauth_refresh_token=attacker")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	w.Handler().ServeHTTP(rr, r)
	require.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, "native-only", cfg.cfg.Integrations["primer"].Credentials["oauth_refresh_token"])
	assert.Equal(t, []string{"primer_read_*"}, cfg.cfg.Integrations["primer"].ToolGlobs)
}
