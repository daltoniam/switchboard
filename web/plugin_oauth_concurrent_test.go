package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginOAuthConcurrentPUTRetainsRotatedTokensOnDisk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var refreshes atomic.Int32
	var issuer *httptest.Server
	issuer = httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			require.NoError(t, json.NewEncoder(rw).Encode(map[string]string{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "userinfo_endpoint": issuer.URL + "/userinfo", "jwks_uri": issuer.URL + "/jwks"}))
		case "/token":
			if refreshes.Add(1) == 1 {
				close(entered)
				<-release
			}
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "rotation-before", r.Form.Get("refresh_token"))
			require.NoError(t, json.NewEncoder(rw).Encode(map[string]any{"access_token": "access-after", "refresh_token": "rotation-after", "token_type": "Bearer", "expires_in": 3600}))
		case "/userinfo":
			require.NoError(t, json.NewEncoder(rw).Encode(map[string]string{"sub": "expected-user", "email": "expected@example.com"}))
		default:
			t.Errorf("unexpected endpoint")
		}
	}))
	t.Cleanup(issuer.Close)
	previous := http.DefaultTransport
	http.DefaultTransport = issuer.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	rt, err := wasm.NewRuntime(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(t.Context()) })
	data, err := os.ReadFile("../wasm/testdata/example.wasm")
	require.NoError(t, err)
	mod, err := rt.LoadModule(t.Context(), data)
	require.NoError(t, err)
	mod.SetName("primer")
	mod.SetConfigService(cfg)
	creds := mcp.Credentials{"oauth_issuer": issuer.URL, "oauth_client_id": "public", "oauth_subject": "expected-user", "oauth_email": "expected@example.com", "oauth_token_key": "api_key", "oauth_refresh_token": "rotation-before", "base_url": "https://api.example"}
	require.NoError(t, cfg.SetIntegration("primer", &mcp.IntegrationConfig{Enabled: true, Credentials: creds, ToolGlobs: []string{"example_echo"}}))
	require.NoError(t, mod.Configure(t.Context(), creds))
	w, reg, _ := setupTestWeb()
	w.services.Config = cfg
	require.NoError(t, reg.Register(mod))
	executed := make(chan error, 1)
	go func() {
		result, err := mod.Execute(t.Context(), "example_echo", map[string]any{"message": "ready"})
		if err == nil {
			assert.False(t, result.IsError)
		}
		executed <- err
	}()
	<-entered
	started, finished := make(chan struct{}), make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rr := httptest.NewRecorder()
		close(started)
		w.Handler().ServeHTTP(rr, localOAuthRequest(http.MethodPut, "/api/integrations/primer/credentials", `{"oauth_refresh_token":"rotation-before","unrelated":"updated"}`))
		finished <- rr
	}()
	<-started
	unblock()
	require.NoError(t, <-executed)
	require.Equal(t, http.StatusOK, (<-finished).Code)
	require.NoError(t, cfg.Load())
	saved, ok := cfg.GetIntegration("primer")
	require.True(t, ok)
	assert.Equal(t, "rotation-after", saved.Credentials["oauth_refresh_token"])
	assert.Equal(t, "updated", saved.Credentials["unrelated"])
	assert.Equal(t, []string{"example_echo"}, saved.ToolGlobs)
	assert.Equal(t, int32(1), refreshes.Load())
}
