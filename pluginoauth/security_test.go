package pluginoauth

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadedTokensCannotFollowChangedSettings(t *testing.T) {
	for _, key := range []string{"oauth_issuer", "oauth_client_id", "oauth_token_key", "oauth_subject", "oauth_email", "oauth_scopes"} {
		t.Run(key, func(t *testing.T) {
			m, p, store := setupManager(t)
			q := start(t, m, p)
			require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
			changed := maps.Clone(store.ic.Credentials)
			changed[key] += "/changed"
			for _, manager := range []*Manager{m, New("primer", store)} {
				require.NoError(t, manager.Load(changed))
				assert.Empty(t, manager.creds["oauth_access_token"])
				assert.Empty(t, manager.creds["oauth_refresh_token"])
				_, err := manager.Credentials(t.Context())
				require.ErrorIs(t, err, ErrReauthorization)
			}
		})
	}
}

func TestInitialExchangeRequiresRefreshToken(t *testing.T) {
	m, p, store := setupManager(t)
	transport := p.server.Client().Transport
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/token" {
			return transport.RoundTrip(r)
		}
		rr := httptest.NewRecorder()
		require.NoError(t, json.NewEncoder(rr).Encode(map[string]any{"access_token": "opaque-access", "token_type": "Bearer", "expires_in": 3600}))
		return rr.Result(), nil
	})
	q := start(t, m, p)
	require.Error(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	assert.Zero(t, store.saves)
	assert.False(t, m.dirty)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestQuarantinedRotationRecoversAfterExpiry(t *testing.T) {
	m, p, store := setupManager(t)
	q := start(t, m, p)
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	before := maps.Clone(store.ic.Credentials)
	now := time.Now().Add(2 * time.Hour)
	m.now = func() time.Time { return now }
	refreshes := 0
	m.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rr := httptest.NewRecorder()
		switch r.URL.Path {
		case "/token":
			require.NoError(t, r.ParseForm())
			refreshes++
			expected := "refresh-rotated"
			if refreshes > 1 {
				expected = "quarantined-refresh"
			}
			assert.Equal(t, expected, r.Form.Get("refresh_token"))
			require.NoError(t, json.NewEncoder(rr).Encode(map[string]any{"access_token": "recovered-access", "refresh_token": "quarantined-refresh", "token_type": "Bearer", "expires_in": 3600}))
		case "/userinfo":
			if refreshes == 1 {
				rr.WriteHeader(http.StatusServiceUnavailable)
			} else {
				require.NoError(t, json.NewEncoder(rr).Encode(map[string]string{"sub": p.user, "email": p.email}))
			}
		default:
			t.Fatalf("unexpected endpoint")
		}
		return rr.Result(), nil
	})
	guest, err := m.Credentials(t.Context())
	require.ErrorIs(t, err, ErrRequest)
	assert.Nil(t, guest)
	assert.Equal(t, before, store.ic.Credentials)
	now = now.Add(2 * time.Hour)
	guest, err = m.Credentials(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, refreshes)
	assert.Equal(t, "recovered-access", guest["tasks_api_key"])
}

func TestExplicitReauthorizationCanReplaceQuarantine(t *testing.T) {
	m, p, store := setupManager(t)
	q := start(t, m, p)
	store.err = errors.New("unavailable")
	require.ErrorIs(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""), ErrPersistence)
	store.err = nil
	raw, err := m.Start(t.Context(), p.credentials(), "http://127.0.0.1:3847/api/integrations/primer/oauth/callback", "browser-cookie")
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	p.challenge = u.Query().Get("code_challenge")
	require.NoError(t, m.Callback(t.Context(), "code", u.Query().Get("state"), "browser-cookie", ""))
	assert.False(t, m.dirty)
}

func TestInvalidBrowserDoesNotConsumeAuthorization(t *testing.T) {
	m, p, _ := setupManager(t)
	q := start(t, m, p)
	require.ErrorIs(t, m.Callback(t.Context(), "code", q.Get("state"), "other-browser", ""), ErrState)
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
}

func TestAbandonedReauthorizationRemainsBlocked(t *testing.T) {
	m, p, store := setupManager(t)
	q := start(t, m, p)
	store.err = errors.New("unavailable")
	require.ErrorIs(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""), ErrPersistence)
	start(t, m, p)
	assert.True(t, m.Active())
	require.NoError(t, m.Load(p.credentials()))
	_, err := m.Credentials(t.Context())
	require.ErrorIs(t, err, ErrReauthorization)
}

func TestCredentialEditCannotUnblockIdentityMismatch(t *testing.T) {
	m, p, store := setupManager(t)
	store.ic.Credentials = p.credentials()
	q := start(t, m, p)
	p.user = "different-user"
	require.ErrorIs(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""), ErrReauthorization)
	require.NoError(t, m.EditCredentials(t.Context(), nil, true))
	assert.Empty(t, store.ic.Credentials["oauth_access_token"])
	assert.Empty(t, store.ic.Credentials["oauth_refresh_token"])
	p.user = "expected-user"
	_, err := m.Credentials(t.Context())
	require.ErrorIs(t, err, ErrReauthorization)
}

func TestCredentialEditDoesNotBypassLoadedBinding(t *testing.T) {
	m, p, store := setupManager(t)
	q := start(t, m, p)
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	store.ic.Credentials["oauth_issuer"] += "/changed"
	restarted := New("primer", store)
	require.NoError(t, restarted.EditCredentials(t.Context(), nil, true))
	require.NoError(t, restarted.Load(store.ic.Credentials))
	assert.Empty(t, restarted.creds["oauth_access_token"])
	assert.Empty(t, restarted.creds["oauth_refresh_token"])
}

func TestReauthorizationSettingsChangeClearsPreviousGuestToken(t *testing.T) {
	m, p, store := setupManager(t)
	store.ic.Credentials = p.credentials()
	store.ic.Credentials["oauth_token_key"] = "old_key"
	store.ic.Credentials["old_key"] = "old-access"
	store.ic.Credentials["tasks_api_key"] = "other-access"
	q := start(t, m, p)
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	assert.Empty(t, store.ic.Credentials["old_key"])
	assert.Empty(t, store.ic.Credentials["tasks_api_key"])
}
