package pluginoauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	ic    *mcp.IntegrationConfig
	err   error
	saves int
	name  string
}

func (s *memoryStore) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	s.name = name
	return s.ic, s.ic != nil
}

func (s *memoryStore) SetIntegration(name string, ic *mcp.IntegrationConfig) error {
	s.name = name
	s.saves++
	if s.err != nil {
		return s.err
	}
	s.ic = ic
	return nil
}

type provider struct {
	server          *httptest.Server
	tokens          int
	refreshes       int
	user            string
	email           string
	tokenError      string
	badEndpoint     string
	userinfoMissing bool
	challenge       string
}

func newProvider(t *testing.T) *provider {
	t.Helper()
	p := &provider{user: "expected-user", email: "expected@example.com"}
	p.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server", "/.well-known/openid-configuration":
			userinfo := p.server.URL + "/userinfo"
			if p.userinfoMissing && r.URL.Path == "/.well-known/oauth-authorization-server" {
				userinfo = ""
			}
			token := p.server.URL + "/token"
			if p.badEndpoint != "" {
				token = p.badEndpoint
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"issuer": p.server.URL, "authorization_endpoint": p.server.URL + "/authorize",
				"token_endpoint": token, "userinfo_endpoint": userinfo, "jwks_uri": p.server.URL + "/jwks",
				"code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none"},
			}))
		case "/token":
			p.tokens++
			require.NoError(t, r.ParseForm())
			assert.Empty(t, r.Form.Get("client_secret"))
			assert.Empty(t, r.Header.Get("Authorization"))
			assert.Equal(t, "public-client", r.Form.Get("client_id"))
			if r.Form.Get("grant_type") == "refresh_token" {
				p.refreshes++
				assert.Equal(t, "refresh-original", r.Form.Get("refresh_token"))
			} else {
				hash := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
				assert.Equal(t, p.challenge, base64.RawURLEncoding.EncodeToString(hash[:]))
				assert.Equal(t, "http://127.0.0.1:3847/api/integrations/primer/oauth/callback", r.Form.Get("redirect_uri"))
			}
			if p.tokenError != "" {
				w.WriteHeader(http.StatusBadRequest)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"error": p.tokenError, "error_description": "secret-provider-details"}))
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"access_token": "opaque-access", "refresh_token": "refresh-rotated", "token_type": "Bearer", "expires_in": 3600, "id_token": "not-an-access-token"}))
		case "/userinfo":
			assert.Contains(t, []string{"Bearer opaque-access", "Bearer loaded-access"}, r.Header.Get("Authorization"))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"sub": p.user, "email": p.email}))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(p.server.Close)
	return p
}

func (p *provider) credentials() mcp.Credentials {
	return mcp.Credentials{"oauth_issuer": p.server.URL, "oauth_client_id": "public-client", "oauth_token_key": "tasks_api_key", "oauth_subject": "expected-user", "oauth_email": "expected@example.com", "base_url": "https://api.example.com"}
}

func setupManager(t *testing.T) (*Manager, *provider, *memoryStore) {
	t.Helper()
	p := newProvider(t)
	store := &memoryStore{ic: &mcp.IntegrationConfig{Enabled: false, Credentials: mcp.Credentials{"unrelated": "keep"}, ToolGlobs: []string{"primer_read_*"}, Identities: map[string]mcp.IntegrationIdentity{"work": {Credentials: mcp.Credentials{"key": "keep"}}}}}
	m := New("primer", store)
	m.client.Transport = p.server.Client().Transport
	return m, p, store
}

func start(t *testing.T, m *Manager, p *provider) url.Values {
	t.Helper()
	raw, err := m.Start(t.Context(), p.credentials(), "http://127.0.0.1:3847/api/integrations/primer/oauth/callback", "browser-cookie")
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()
	p.challenge = q.Get("code_challenge")
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Contains(t, q.Get("scope"), "offline_access")
	assert.Empty(t, q.Get("code_verifier"))
	return q
}

func TestAuthorizationCode(t *testing.T) {
	m, p, store := setupManager(t)
	p.userinfoMissing = true
	q := start(t, m, p)
	assert.Zero(t, store.saves)
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", p.server.URL))
	assert.Equal(t, "primer", store.name)
	assert.False(t, store.ic.Enabled)
	assert.Equal(t, []string{"primer_read_*"}, store.ic.ToolGlobs)
	assert.Equal(t, "keep", store.ic.Credentials["unrelated"])
	assert.Equal(t, "keep", store.ic.Identities["work"].Credentials["key"])
	assert.Equal(t, "refresh-rotated", store.ic.Credentials["oauth_refresh_token"])
	guest, err := m.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "opaque-access", guest["tasks_api_key"])
	for key := range guest {
		assert.False(t, strings.HasPrefix(key, "oauth_"))
	}
	assert.ErrorIs(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", p.server.URL), ErrState)
	assert.Equal(t, 1, p.tokens)
}

func TestStateValidation(t *testing.T) {
	for _, name := range []string{"mismatch", "browser", "expired", "issuer", "denied"} {
		t.Run(name, func(t *testing.T) {
			m, p, store := setupManager(t)
			q := start(t, m, p)
			state, browser, issuer, code := q.Get("state"), "browser-cookie", p.server.URL, "code"
			switch name {
			case "mismatch":
				state = "other"
			case "browser":
				browser = "other"
			case "expired":
				m.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
			case "issuer":
				issuer = "https://evil.example"
			case "denied":
				code = ""
			}
			require.Error(t, m.Callback(t.Context(), code, state, browser, issuer))
			assert.Zero(t, store.saves)
			assert.Zero(t, p.tokens)
		})
	}
}

func TestRefreshRotationPersistenceRetryAndRestart(t *testing.T) {
	m, p, store := setupManager(t)
	creds := p.credentials()
	creds["oauth_access_token"] = "loaded-access"
	creds["oauth_refresh_token"] = "refresh-original"
	creds["oauth_expires_at"] = time.Now().Add(30 * time.Second).Format(time.RFC3339)
	require.NoError(t, m.Load(creds))
	store.err = errors.New("secret-storage-details")
	_, err := m.Credentials(t.Context())
	require.ErrorIs(t, err, ErrPersistence)
	assert.NotContains(t, err.Error(), "secret")
	assert.Equal(t, 1, p.refreshes)
	_, err = m.Credentials(t.Context())
	require.ErrorIs(t, err, ErrPersistence)
	assert.Equal(t, 1, p.refreshes)
	store.err = nil
	require.NoError(t, m.Load(creds))
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			guest, err := m.Credentials(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, "opaque-access", guest["tasks_api_key"])
		})
	}
	wg.Wait()
	assert.Equal(t, 1, p.refreshes)
	assert.Equal(t, "refresh-rotated", store.ic.Credentials["oauth_refresh_token"])
	restarted := New("primer", store)
	restarted.client.Transport = p.server.Client().Transport
	require.NoError(t, restarted.Load(store.ic.Credentials))
	guest, err := restarted.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "opaque-access", guest["tasks_api_key"])
	assert.Equal(t, 1, p.refreshes)
}

func TestWrongUserNeverPersists(t *testing.T) {
	for _, mode := range []string{"initial-subject", "initial-email", "refresh-subject", "refresh-email"} {
		t.Run(mode, func(t *testing.T) {
			m, p, store := setupManager(t)
			before := maps.Clone(store.ic.Credentials)
			if strings.HasSuffix(mode, "subject") {
				p.user = "wrong-user"
			} else {
				p.email = "wrong@example.com"
			}
			var err error
			if strings.HasPrefix(mode, "initial") {
				q := start(t, m, p)
				err = m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", "")
			} else {
				creds := p.credentials()
				creds["oauth_refresh_token"] = "refresh-original"
				require.NoError(t, m.Load(creds))
				_, err = m.Credentials(t.Context())
			}
			require.ErrorIs(t, err, ErrReauthorization)
			assert.Zero(t, store.saves)
			assert.Equal(t, before, store.ic.Credentials)
		})
	}
}

func TestInvalidGrantFailsClosed(t *testing.T) {
	m, p, _ := setupManager(t)
	p.tokenError = "invalid_grant"
	creds := p.credentials()
	creds["oauth_refresh_token"] = "refresh-original"
	require.NoError(t, m.Load(creds))
	for range 2 {
		_, err := m.Credentials(t.Context())
		require.ErrorIs(t, err, ErrReauthorization)
		assert.NotContains(t, err.Error(), "secret-provider-details")
		require.NoError(t, m.Load(creds))
	}
	assert.Equal(t, 1, p.refreshes)
}

func TestUnsafeConfiguration(t *testing.T) {
	for _, issuer := range []string{"http://issuer.example", "https://user:secret@issuer.example", "https://issuer.example?secret=x", "https://issuer.example/#fragment"} {
		t.Run(issuer, func(t *testing.T) {
			m, p, _ := setupManager(t)
			creds := p.credentials()
			creds["oauth_issuer"] = issuer
			require.ErrorIs(t, m.Load(creds), ErrConfiguration)
		})
	}
	for _, endpoint := range []string{"http://issuer.example/token", "https://other.example/token", "https://secret@other.example/token"} {
		t.Run(endpoint, func(t *testing.T) {
			m, p, _ := setupManager(t)
			p.badEndpoint = endpoint
			_, err := m.Start(t.Context(), p.credentials(), "http://127.0.0.1:3847/api/integrations/primer/oauth/callback", "cookie")
			require.ErrorIs(t, err, ErrDiscovery)
		})
	}
}

func TestInitialPersistenceRetryWithPreviousSettings(t *testing.T) {
	m, p, store := setupManager(t)
	previous := p.credentials()
	previous["oauth_client_id"] = "previous-client"
	require.NoError(t, m.Load(previous))
	q := start(t, m, p)
	store.err = errors.New("disk full")
	require.ErrorIs(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""), ErrPersistence)
	require.NoError(t, m.Load(previous))
	store.err = nil
	guest, err := m.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "opaque-access", guest["tasks_api_key"])
	assert.Equal(t, "public-client", store.ic.Credentials["oauth_client_id"])
}

func TestReauthorizeAfterInvalidGrant(t *testing.T) {
	m, p, store := setupManager(t)
	creds := p.credentials()
	creds["oauth_refresh_token"] = "refresh-original"
	require.NoError(t, m.Load(creds))
	p.tokenError = "invalid_grant"
	_, err := m.Credentials(t.Context())
	require.ErrorIs(t, err, ErrReauthorization)
	p.tokenError = ""
	q := start(t, m, p)
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	guest, err := m.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "opaque-access", guest["tasks_api_key"])
	assert.Equal(t, "refresh-rotated", store.ic.Credentials["oauth_refresh_token"])
}
