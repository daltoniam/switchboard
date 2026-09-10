package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/pluginoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginOAuthRotatedTokensSurviveDiskReload(t *testing.T) {
	var refreshes atomic.Int32
	var issuer *httptest.Server
	issuer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "userinfo_endpoint": issuer.URL + "/userinfo", "jwks_uri": issuer.URL + "/jwks"}))
		case "/token":
			refreshes.Add(1)
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "refresh-before", r.Form.Get("refresh_token"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"access_token": "access-after", "refresh_token": "refresh-after", "expires_in": 3600, "token_type": "Bearer"}))
		case "/userinfo":
			assert.Equal(t, "Bearer access-after", r.Header.Get("Authorization"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"sub": "expected", "email": "expected@example.com"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer issuer.Close()
	previousTransport := http.DefaultTransport
	http.DefaultTransport = issuer.Client().Transport
	defer func() { http.DefaultTransport = previousTransport }()
	store, path := newTestManager(t)
	require.NoError(t, store.Load())
	creds := mcp.Credentials{"oauth_issuer": issuer.URL, "oauth_client_id": "public", "oauth_token_key": "tasks_api_key", "oauth_subject": "expected", "oauth_email": "expected@example.com", "oauth_refresh_token": "refresh-before", "oauth_expires_at": time.Now().Format(time.RFC3339), "unrelated": "preserved"}
	ic := &mcp.IntegrationConfig{Enabled: false, Credentials: creds, ToolGlobs: []string{"primer_read_*"}, Identities: map[string]mcp.IntegrationIdentity{"work": {Credentials: mcp.Credentials{"key": "identity-key"}}}}
	require.NoError(t, store.SetIntegration("primer", ic))
	oauth := pluginoauth.New("primer", store)
	require.NoError(t, oauth.Load(creds))
	guest, err := oauth.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "access-after", guest["tasks_api_key"])
	durable := &manager{filePath: path, envLookup: noEnv}
	require.NoError(t, durable.Load())
	saved, ok := durable.GetIntegration("primer")
	require.True(t, ok)
	assert.False(t, saved.Enabled)
	assert.Equal(t, ic.ToolGlobs, saved.ToolGlobs)
	assert.Equal(t, ic.Identities, saved.Identities)
	assert.Equal(t, "preserved", saved.Credentials["unrelated"])
	assert.Equal(t, "refresh-after", saved.Credentials["oauth_refresh_token"])
	assert.Equal(t, "access-after", saved.Credentials["oauth_access_token"])
	restarted := pluginoauth.New("primer", durable)
	require.NoError(t, restarted.Load(saved.Credentials))
	guest, err = restarted.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "access-after", guest["tasks_api_key"])
	assert.Equal(t, int32(1), refreshes.Load())
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
