package remotemcp

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var renewalNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

// tokenServer is an OAuth server whose refresh tokens are single use, like
// Higgsfield's: each refresh returns a new refresh token, and reusing an old
// one fails.
type tokenServer struct {
	*httptest.Server
	mu        sync.Mutex
	current   string
	issued    int
	refreshes atomic.Int32
	forms     []url.Values
	fail      bool
}

func newTokenServer(t *testing.T, refreshToken string) *tokenServer {
	t.Helper()
	ts := &tokenServer{current: refreshToken}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": ts.URL + "/authorize",
				"token_endpoint":         ts.URL + "/token",
			})
		case "/token":
			ts.refreshes.Add(1)
			require.NoError(t, r.ParseForm())
			ts.mu.Lock()
			defer ts.mu.Unlock()
			ts.forms = append(ts.forms, r.PostForm)
			if ts.fail || r.PostForm.Get("refresh_token") != ts.current {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			ts.issued++
			ts.current = "refresh-" + string(rune('0'+ts.issued))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-" + string(rune('0'+ts.issued)),
				"refresh_token": ts.current,
				"expires_in":    86400,
				"token_type":    "Bearer",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// memoryStore stands in for the shared config file. Its lock is shared by
// every TokenRenewal given the same store, like processes sharing a file.
type memoryStore struct {
	mu    sync.Mutex
	creds mcp.Credentials
}

func (s *memoryStore) UpdateIntegrationShared(_ context.Context, name string, update func(*mcp.IntegrationConfig) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ic := &mcp.IntegrationConfig{Enabled: true, Credentials: maps.Clone(s.creds)}
	if err := update(ic); err != nil {
		return err
	}
	s.creds = ic.Credentials
	return nil
}

func (s *memoryStore) get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.creds[key]
}

func loginExpiring(in time.Duration) mcp.Credentials {
	return mcp.Credentials{
		CredKeyAccessToken:  "access-0",
		CredKeyRefreshToken: "refresh-0",
		CredKeyTokenExpiry:  formatExpiry(renewalNow.Add(in)),
		CredKeyClientID:     "client-1",
		"base_url":          "",
	}
}

func newRenewal(store mcp.SharedIntegrationConfigUpdater) *TokenRenewal {
	r := &TokenRenewal{store: store}
	r.now = func() time.Time { return renewalNow }
	return r
}

func target(ts *tokenServer) RenewalTarget {
	return RenewalTarget{Integration: "higgsfield", ServerURL: ts.URL, Resource: ts.URL + "/mcp"}
}

func TestTokenRenewal_RenewsNearExpiryAndSavesRotatedTokens(t *testing.T) {
	ts := newTokenServer(t, "refresh-0")
	creds := loginExpiring(30 * time.Minute)
	store := &memoryStore{creds: maps.Clone(creds)}

	fresh, renewed, err := newRenewal(store).Fresh(context.Background(), target(ts), creds)

	require.NoError(t, err)
	assert.True(t, renewed)
	assert.Equal(t, "access-1", fresh[CredKeyAccessToken])
	assert.Equal(t, "refresh-1", fresh[CredKeyRefreshToken])
	assert.Equal(t, formatExpiry(renewalNow.Add(24*time.Hour)), fresh[CredKeyTokenExpiry])
	assert.Equal(t, "", fresh["base_url"], "other credentials are kept")
	for _, key := range []string{CredKeyAccessToken, CredKeyRefreshToken, CredKeyTokenExpiry} {
		assert.Equal(t, fresh[key], store.get(key), "%s is saved", key)
	}
	require.Len(t, ts.forms, 1)
	form := ts.forms[0]
	assert.Equal(t, "refresh_token", form.Get("grant_type"))
	assert.Equal(t, "client-1", form.Get("client_id"))
	assert.Equal(t, ts.URL+"/mcp", form.Get("resource"))
}

func TestTokenRenewal_LeavesTokenAlone(t *testing.T) {
	tests := []struct {
		name  string
		creds mcp.Credentials
	}{
		{"fresh token", loginExpiring(2 * time.Hour)},
		{"no refresh token", func() mcp.Credentials {
			c := loginExpiring(time.Minute)
			delete(c, CredKeyRefreshToken)
			return c
		}()},
		{"unknown expiry", func() mcp.Credentials {
			c := loginExpiring(time.Minute)
			c[CredKeyTokenExpiry] = ""
			return c
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTokenServer(t, "refresh-0")
			store := &memoryStore{creds: maps.Clone(tt.creds)}

			fresh, renewed, err := newRenewal(store).Fresh(context.Background(), target(ts), tt.creds)

			require.NoError(t, err)
			assert.False(t, renewed)
			assert.Equal(t, tt.creds, fresh)
			assert.Zero(t, ts.refreshes.Load(), "no token request")
		})
	}
}

// Two processes find the token near expiry at once. The single-use refresh
// token must be spent once; the second process adopts what the first saved.
func TestTokenRenewal_LockPreventsDoubleRefresh(t *testing.T) {
	ts := newTokenServer(t, "refresh-0")
	creds := loginExpiring(10 * time.Minute)
	store := &memoryStore{creds: maps.Clone(creds)}

	var wg sync.WaitGroup
	results := make([]mcp.Credentials, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fresh, _, err := newRenewal(store).Fresh(context.Background(), target(ts), creds)
			assert.NoError(t, err)
			results[i] = fresh
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), ts.refreshes.Load())
	for _, fresh := range results {
		assert.Equal(t, "access-1", fresh[CredKeyAccessToken])
	}
	assert.Equal(t, "refresh-1", store.get(CredKeyRefreshToken))
}

// A process holding an old copy must use the newest saved refresh token, not
// its own spent one.
func TestTokenRenewal_UsesNewestSavedRefreshToken(t *testing.T) {
	ts := newTokenServer(t, "refresh-0")
	stale := loginExpiring(10 * time.Minute)
	store := &memoryStore{creds: maps.Clone(stale)}
	_, _, err := newRenewal(store).Fresh(context.Background(), target(ts), stale)
	require.NoError(t, err)

	// A day later both copies are due; the stale process renews next.
	later := newRenewal(store)
	later.now = func() time.Time { return renewalNow.Add(24 * time.Hour) }
	fresh, renewed, err := later.Fresh(context.Background(), target(ts), stale)

	require.NoError(t, err)
	assert.True(t, renewed)
	assert.Equal(t, "access-2", fresh[CredKeyAccessToken])
	assert.Equal(t, "refresh-1", ts.forms[1].Get("refresh_token"))
}

func TestTokenRenewal_FailureIsClearAndSavesNothing(t *testing.T) {
	ts := newTokenServer(t, "refresh-0")
	ts.fail = true
	creds := loginExpiring(10 * time.Minute)
	store := &memoryStore{creds: maps.Clone(creds)}

	_, renewed, err := newRenewal(store).Fresh(context.Background(), target(ts), creds)

	require.Error(t, err)
	assert.False(t, renewed)
	assert.Contains(t, err.Error(), "higgsfield")
	assert.Contains(t, err.Error(), "invalid_grant")
	assert.Contains(t, err.Error(), "sign in")
	assert.Equal(t, creds, store.creds)
}

func TestTokenRenewal_WithoutStoreDoesNotRefresh(t *testing.T) {
	ts := newTokenServer(t, "refresh-0")
	creds := loginExpiring(10 * time.Minute)

	fresh, renewed, err := newRenewal(nil).Fresh(context.Background(), target(ts), creds)

	require.NoError(t, err, "a token that cannot be saved is never spent")
	assert.False(t, renewed)
	assert.Equal(t, creds, fresh)
	assert.Zero(t, ts.refreshes.Load())
}

func TestTokenRenewal_KeepsRefreshTokenWhenServerDoesNotRotate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-1", "expires_in": 3600})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"authorization_endpoint": "x", "token_endpoint": "http://" + r.Host + "/token"})
	}))
	defer srv.Close()
	creds := loginExpiring(10 * time.Minute)
	store := &memoryStore{creds: maps.Clone(creds)}

	fresh, _, err := newRenewal(store).Fresh(context.Background(), RenewalTarget{Integration: "higgsfield", ServerURL: srv.URL}, creds)

	require.NoError(t, err)
	assert.Equal(t, "refresh-0", fresh[CredKeyRefreshToken])
	assert.Equal(t, formatExpiry(renewalNow.Add(time.Hour)), fresh[CredKeyTokenExpiry])
}

func TestHandleOAuthCallback_KeepsRefreshTokenAndExpiry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": "http://" + r.Host + "/authorize",
				"token_endpoint":         "http://" + r.Host + "/token",
				"registration_endpoint":  "http://" + r.Host + "/register",
			})
		case "/register":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"client_id":"client-1"}`))
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","expires_in":86400}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	authorizeURL, err := StartOAuth("renewal-callback", srv.URL, "http://localhost/callback")
	require.NoError(t, err)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)

	before := time.Now()
	require.NoError(t, HandleOAuthCallback("renewal-callback", "code-1", parsed.Query().Get("state")))

	creds, ok := LoginCredentials("renewal-callback")
	require.True(t, ok)
	assert.Equal(t, "access-1", creds[CredKeyAccessToken])
	assert.Equal(t, "refresh-1", creds[CredKeyRefreshToken])
	assert.Equal(t, "client-1", creds[CredKeyClientID])
	expiry, err := time.Parse(time.RFC3339, creds[CredKeyTokenExpiry])
	require.NoError(t, err)
	assert.WithinDuration(t, before.Add(24*time.Hour), expiry, time.Minute)
	issued, err := time.Parse(time.RFC3339, creds[CredKeyTokenIssued])
	require.NoError(t, err)
	assert.WithinDuration(t, before, issued, time.Minute)
}

func TestTokenRenewal_ShortLivedTokenIsNotRenewedAgainRightAway(t *testing.T) {
	var refreshes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			n := refreshes.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-" + string(rune('0'+n)), "expires_in": 1800})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"authorization_endpoint": "x", "token_endpoint": "http://" + r.Host + "/token"})
	}))
	defer srv.Close()
	creds := loginExpiring(10 * time.Minute)
	store := &memoryStore{creds: maps.Clone(creds)}
	renewal := newRenewal(store)
	tgt := RenewalTarget{Integration: "higgsfield", ServerURL: srv.URL}

	fresh, renewed, err := renewal.Fresh(context.Background(), tgt, creds)
	require.NoError(t, err)
	require.True(t, renewed)
	assert.Equal(t, formatExpiry(renewalNow), fresh[CredKeyTokenIssued])

	again, renewed, err := renewal.Fresh(context.Background(), tgt, fresh)
	require.NoError(t, err)
	assert.False(t, renewed, "a token just issued for 30 minutes is not due yet")
	assert.Equal(t, fresh, again)
	assert.EqualValues(t, 1, refreshes.Load())
}

func TestRenewalDue(t *testing.T) {
	login := func(issuedAgo, expiresIn time.Duration) mcp.Credentials {
		c := loginExpiring(expiresIn)
		if issuedAgo >= 0 {
			c[CredKeyTokenIssued] = formatExpiry(renewalNow.Add(-issuedAgo))
		}
		return c
	}
	tests := []struct {
		name  string
		creds mcp.Credentials
		want  bool
	}{
		{"long token far from expiry", login(time.Hour, 23*time.Hour), false},
		{"long token inside the lead", login(23*time.Hour+30*time.Minute, 30*time.Minute), true},
		{"short token just issued", login(0, 30*time.Minute), false},
		{"short token past half its life", login(20*time.Minute, 10*time.Minute), true},
		{"unknown issue time uses the full lead", login(-1, 30*time.Minute), true},
		{"expired", login(25*time.Hour, -time.Hour), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, renewalDue(tt.creds, renewalNow))
		})
	}
}
