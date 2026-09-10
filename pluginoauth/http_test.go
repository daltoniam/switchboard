package pluginoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderResponseBounds(t *testing.T) {
	for _, name := range []string{"redirect", "oversize", "malformed", "timeout", "id-token-only", "missing-expiry"} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch name {
				case "redirect":
					http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
				case "oversize":
					_, _ = w.Write([]byte(strings.Repeat("x", maxBody+1)))
				case "malformed":
					_, _ = w.Write([]byte("secret-invalid-json"))
				case "timeout":
					time.Sleep(150 * time.Millisecond)
				case "id-token-only":
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"id_token": "secret-id-token", "token_type": "Bearer", "expires_in": 3600}))
				case "missing-expiry":
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"access_token": "secret-access-token", "token_type": "Bearer"}))
				}
			}))
			defer server.Close()
			m := New("primer", nil)
			m.client.Transport = server.Client().Transport
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			_, err := m.exchange(ctx, server.URL+"/token", url.Values{"grant_type": {"refresh_token"}})
			require.ErrorIs(t, err, ErrRequest)
			assert.NotContains(t, err.Error(), "secret")
			assert.Equal(t, int32(1), calls.Load())
		})
	}
}

func TestMetadataValidation(t *testing.T) {
	for _, field := range []string{"issuer", "authorization", "token", "userinfo", "jwks"} {
		t.Run(field, func(t *testing.T) {
			md := metadata{Issuer: "https://issuer.example", AuthorizationEndpoint: "https://issuer.example/authorize", TokenEndpoint: "https://issuer.example/token", UserinfoEndpoint: "https://issuer.example/userinfo", JWKSURI: "https://issuer.example/jwks"}
			switch field {
			case "issuer":
				md.Issuer = "https://other.example"
			case "authorization":
				md.AuthorizationEndpoint = "https://other.example/authorize"
			case "token":
				md.TokenEndpoint = "https://issuer.example:8443/token"
			case "userinfo":
				md.UserinfoEndpoint = "http://issuer.example/userinfo"
			case "jwks":
				md.JWKSURI = "https://issuer.example/jwks?secret=x"
			}
			assert.ErrorIs(t, validateMetadata("https://issuer.example", md, true), ErrDiscovery)
		})
	}
}

func TestStateIsolationAndReplacement(t *testing.T) {
	m, p, store := setupManager(t)
	first := start(t, m, p)
	second := start(t, m, p)
	require.ErrorIs(t, m.Callback(t.Context(), "code", first.Get("state"), "browser-cookie", ""), ErrState)
	other := New("other", store)
	assert.ErrorIs(t, other.Callback(t.Context(), "code", second.Get("state"), "browser-cookie", ""), ErrState)
	require.NoError(t, m.Callback(t.Context(), "code", second.Get("state"), "browser-cookie", ""))
	assert.Equal(t, 1, p.tokens)
}

func TestPendingAuthorizationSurvivesInitialLoad(t *testing.T) {
	m, p, _ := setupManager(t)
	q := start(t, m, p)
	require.NoError(t, m.Load(p.credentials()))
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
}

func TestAuthorizationPreservesConcurrentUnrelatedCredentialUpdate(t *testing.T) {
	m, p, store := setupManager(t)
	store.ic.Credentials["base_url"] = "https://api.example.com"
	q := start(t, m, p)
	store.ic.Credentials["base_url"] = "https://updated.example.com"
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	assert.Equal(t, "https://updated.example.com", store.ic.Credentials["base_url"])
}
