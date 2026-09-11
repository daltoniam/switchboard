package switchboard

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
	"github.com/daltoniam/switchboard/registry"
	"github.com/daltoniam/switchboard/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type credentialSnapshotBarrier struct {
	mcp.ConfigService
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (s *credentialSnapshotBarrier) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	ic, ok := s.ConfigService.GetIntegration(name)
	s.once.Do(func() { close(s.entered); <-s.release })
	return ic, ok
}

func TestConfigureIntegrationConcurrentRotation(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		enabled, changeIssuer bool
	}{
		{name: "enabled", enabled: true},
		{name: "disabled"},
		{name: "issuer changed", enabled: true, changeIssuer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			store, err := config.NewManager()
			require.NoError(t, err)
			var refreshes atomic.Int32
			var issuer *httptest.Server
			issuer = httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/oauth-authorization-server":
					_ = json.NewEncoder(rw).Encode(map[string]string{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "userinfo_endpoint": issuer.URL + "/userinfo", "jwks_uri": issuer.URL + "/jwks"})
				case "/token":
					refreshes.Add(1)
					_ = json.NewEncoder(rw).Encode(map[string]any{"access_token": "access-after", "refresh_token": "rotation-after", "token_type": "Bearer", "expires_in": 3600})
				case "/userinfo":
					_ = json.NewEncoder(rw).Encode(map[string]string{"sub": "expected-user", "email": "expected@example.com"})
				default:
					http.NotFound(rw, r)
				}
			}))
			t.Cleanup(issuer.Close)
			previous := http.DefaultTransport
			http.DefaultTransport = issuer.Client().Transport
			t.Cleanup(func() { http.DefaultTransport = previous })
			rt, err := wasm.NewRuntime(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() { _ = rt.Close(t.Context()) })
			data, err := os.ReadFile("../../wasm/testdata/example.wasm")
			require.NoError(t, err)
			mod, err := rt.LoadModule(t.Context(), data)
			require.NoError(t, err)
			mod.SetName("primer")
			mod.SetConfigService(store)
			creds := mcp.Credentials{"oauth_issuer": issuer.URL, "oauth_client_id": "public", "oauth_subject": "expected-user", "oauth_email": "expected@example.com", "oauth_token_key": "api_key", "oauth_refresh_token": "rotation-before", "base_url": "https://api.example"}
			require.NoError(t, store.SetIntegration("primer", &mcp.IntegrationConfig{Enabled: true, Credentials: creds, ToolGlobs: []string{"example_echo"}}))
			require.NoError(t, mod.Configure(t.Context(), creds))
			barrier := &credentialSnapshotBarrier{ConfigService: store, entered: make(chan struct{}), release: make(chan struct{})}
			var release sync.Once
			unblock := func() { release.Do(func() { close(barrier.release) }) }
			defer unblock()
			reg := registry.New()
			require.NoError(t, reg.Register(mod))
			s := &switchboardInt{services: &mcp.Services{Config: barrier, Registry: reg}}
			updates := map[string]any{"unrelated": "updated", "oauth_refresh_token": "rotation-before"}
			if tc.changeIssuer {
				updates["oauth_issuer"] = "https://new.example"
			}
			done := make(chan *mcp.ToolResult, 1)
			go func() {
				result, err := configureIntegration(t.Context(), s, map[string]any{"name": "primer", "enabled": tc.enabled, "credentials": updates})
				assert.NoError(t, err)
				done <- result
			}()
			<-barrier.entered
			result, err := mod.Execute(t.Context(), "example_echo", map[string]any{"message": "ready"})
			unblock()
			require.NoError(t, err)
			require.False(t, result.IsError)
			result = <-done
			require.False(t, result.IsError, result.Data)
			require.NoError(t, store.Load())
			saved, _ := store.GetIntegration("primer")
			if tc.changeIssuer {
				assert.Equal(t, "https://new.example", saved.Credentials["oauth_issuer"])
				assert.Empty(t, saved.Credentials["oauth_refresh_token"])
				assert.Empty(t, saved.Credentials["oauth_access_token"])
			} else {
				assert.Equal(t, "rotation-after", saved.Credentials["oauth_refresh_token"])
				assert.Equal(t, "access-after", saved.Credentials["oauth_access_token"])
			}
			assert.Equal(t, tc.enabled, saved.Enabled)
			assert.Equal(t, "updated", saved.Credentials["unrelated"])
			assert.Equal(t, []string{"example_echo"}, saved.ToolGlobs)
			assert.Equal(t, int32(1), refreshes.Load())
		})
	}
}
