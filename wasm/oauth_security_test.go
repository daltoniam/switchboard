package wasm

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dirtyOAuthModule(t *testing.T) (*Loader, string, *loaderConfig, *Module) {
	t.Helper()
	refreshes := 0
	t.Cleanup(func() { assert.Equal(t, 1, refreshes) })
	var issuer *httptest.Server
	issuer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "userinfo_endpoint": issuer.URL + "/userinfo", "jwks_uri": issuer.URL + "/jwks"}))
		case "/token":
			refreshes++
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "rotation-before", r.Form.Get("refresh_token"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"access_token": "access-after", "refresh_token": "rotation-after", "token_type": "Bearer", "expires_in": 3600}))
		case "/userinfo":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"sub": "expected-user", "email": "expected@example.com"}))
		default:
			t.Errorf("unexpected endpoint")
		}
	}))
	t.Cleanup(issuer.Close)
	previous := http.DefaultTransport
	http.DefaultTransport = issuer.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	creds := oauthCredentials()
	creds["oauth_issuer"], creds["api_key"] = issuer.URL, ""
	creds["oauth_refresh_token"], creds["oauth_expires_at"] = "rotation-before", time.Now().Format(time.RFC3339)
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{"example": {Enabled: true, Credentials: creds}})
	loader, path := newLoaderForTest(t, cfg)
	require.NoError(t, loader.LoadPlugin(t.Context(), path, ""))
	mod := loader.modules["example"]
	cfg.setErr = errors.New("storage unavailable")
	_, err := mod.oauth.Credentials(t.Context())
	require.Error(t, err)
	return loader, path, cfg, mod
}

func TestOAuthCloseAndUnloadPreserveUnsavedRotation(t *testing.T) {
	for _, action := range []string{"close", "unload", "reload"} {
		t.Run(action, func(t *testing.T) {
			loader, path, cfg, mod := dirtyOAuthModule(t)
			var err error
			switch action {
			case "close":
				err = mod.Close(t.Context())
			case "unload":
				err = loader.UnloadPlugin(t.Context(), "example")
			case "reload":
				err = loader.LoadPlugin(t.Context(), path, "")
			}
			require.Error(t, err)
			assert.False(t, mod.closed.Load())
			assert.Same(t, mod, loader.modules["example"])
			registered, ok := loader.reg.Get("example")
			require.True(t, ok)
			assert.Same(t, mod, registered)
			cfg.setErr = nil
			require.NoError(t, mod.Close(t.Context()))
			assert.Equal(t, "rotation-after", cfg.cfg.Integrations["example"].Credentials["oauth_refresh_token"])
		})
	}
}

func TestOAuthReloadReadsCredentialsAfterOldFlush(t *testing.T) {
	loader, path, cfg, old := dirtyOAuthModule(t)
	cfg.setErr = nil
	require.NoError(t, loader.LoadPlugin(t.Context(), path, ""))
	assert.True(t, old.closed.Load())
	replacement := loader.modules["example"]
	require.NotSame(t, old, replacement)
	require.NotNil(t, replacement.oauth)
	guest, err := replacement.oauth.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "access-after", guest["api_key"])
	assert.Equal(t, "rotation-after", cfg.cfg.Integrations["example"].Credentials["oauth_refresh_token"])
}
