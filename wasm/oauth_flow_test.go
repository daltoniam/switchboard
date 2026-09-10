package wasm

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthModuleCodeRefreshAndGuestHTTP(t *testing.T) {
	var tokens, apiCalls atomic.Int32
	var challenge string
	var issuer *httptest.Server
	issuer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "userinfo_endpoint": issuer.URL + "/userinfo", "jwks_uri": issuer.URL + "/jwks"}))
		case "/token":
			tokens.Add(1)
			require.NoError(t, r.ParseForm())
			assert.Empty(t, r.Form.Get("client_secret"))
			access, refresh, expires := "access-initial", "refresh-initial", 30
			if r.Form.Get("grant_type") == "refresh_token" {
				assert.Equal(t, "refresh-initial", r.Form.Get("refresh_token"))
				access, refresh, expires = "access-refreshed", "refresh-rotated", 3600
			} else {
				hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
				assert.Equal(t, challenge, base64.RawURLEncoding.EncodeToString(hash[:]))
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"access_token": access, "refresh_token": refresh, "expires_in": expires, "token_type": "Bearer"}))
		case "/userinfo":
			assert.Contains(t, []string{"Bearer access-initial", "Bearer access-refreshed"}, r.Header.Get("Authorization"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"sub": "expected-user", "email": "expected@example.com"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer issuer.Close()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = issuer.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		assert.Equal(t, "Bearer access-refreshed", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer api.Close()
	creds := oauthCredentials()
	creds["oauth_issuer"], creds["base_url"] = issuer.URL, api.URL
	cfg := newLoaderConfig(map[string]*mcp.IntegrationConfig{"example": {Enabled: true, Credentials: creds, ToolGlobs: []string{"example_*"}}})
	mod := loadTestModule(t)
	mod.SetConfigService(cfg)
	require.NoError(t, mod.Configure(t.Context(), creds))
	raw, err := mod.StartOAuth(t.Context(), creds, "http://127.0.0.1:3847/api/integrations/example/oauth/callback", "browser")
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	challenge = u.Query().Get("code_challenge")
	cfg.setErr = errors.New("disk full")
	require.Error(t, mod.CompleteOAuth(t.Context(), "code", u.Query().Get("state"), "browser", issuer.URL))
	require.NoError(t, mod.Configure(t.Context(), creds))
	cfg.setErr = nil
	assert.False(t, mod.Healthy(t.Context()))
	assert.Equal(t, int32(2), tokens.Load())
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			result, err := mod.Execute(t.Context(), "example_list_items", map[string]any{})
			assert.NoError(t, err)
			assert.True(t, result.IsError)
		})
	}
	wg.Wait()
	assert.Equal(t, int32(2), tokens.Load())
	assert.Equal(t, int32(5), apiCalls.Load(), "401 responses must not replay API calls")
	assert.Equal(t, "refresh-rotated", cfg.cfg.Integrations["example"].Credentials["oauth_refresh_token"])
	memory, ok := mod.mod.Memory().Read(0, mod.mod.Memory().Size())
	require.True(t, ok)
	for _, secret := range []string{"refresh-initial", "refresh-rotated", "oauth_refresh_token", "code_verifier"} {
		assert.False(t, strings.Contains(string(memory), secret))
	}
	restarted := loadTestModule(t)
	restarted.SetConfigService(cfg)
	require.NoError(t, restarted.Configure(t.Context(), cfg.cfg.Integrations["example"].Credentials))
	_, err = restarted.Execute(t.Context(), "example_list_items", map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, int32(2), tokens.Load())
	assert.Equal(t, int32(6), apiCalls.Load())
}

func TestOAuthIncompleteConfigurationFailsClosed(t *testing.T) {
	mod := loadTestModule(t)
	require.NoError(t, mod.Configure(t.Context(), mcp.Credentials{"base_url": "https://api.example.com", "api_key": "generic"}))
	err := mod.Configure(t.Context(), mcp.Credentials{"oauth_refresh_token": "native-secret-without-issuer", "api_key": "generic", "base_url": "https://api.example.com"})
	require.Error(t, err)
	result, err := mod.Execute(t.Context(), "example_echo", map[string]any{"message": "must-not-run"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	memory, ok := mod.mod.Memory().Read(0, mod.mod.Memory().Size())
	require.True(t, ok)
	assert.False(t, strings.Contains(string(memory), "native-secret-without-issuer"))
}
