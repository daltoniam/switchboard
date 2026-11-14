package higgsfield

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/remotemcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sharedStore is a config service that only supports shared updates, which
// is all renewal uses.
type sharedStore struct {
	mcp.ConfigService
	mu    sync.Mutex
	creds mcp.Credentials
}

func (s *sharedStore) UpdateIntegrationShared(_ context.Context, name string, update func(*mcp.IntegrationConfig) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name != integrationName {
		panic("unexpected integration " + name)
	}
	ic := &mcp.IntegrationConfig{Enabled: true, Credentials: maps.Clone(s.creds)}
	if err := update(ic); err != nil {
		return err
	}
	s.creds = ic.Credentials
	return nil
}

// withTokenEndpoint makes the fixture an OAuth server that renews
// "refresh-0" into the fixture's access token, or rejects every refresh.
func withTokenEndpoint(f *hostedFixture, reject bool) {
	f.oauth = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"authorization_endpoint": f.server.URL + "/authorize",
				"token_endpoint":         f.server.URL + "/token",
			})
		case "/token":
			if reject || r.FormValue("refresh_token") != "refresh-0" || r.FormValue("resource") != f.server.URL+"/mcp" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"` + fixtureToken + `","refresh_token":"refresh-1","expires_in":86400}`))
		default:
			http.NotFound(w, r)
		}
	})
}

func expiringLogin(baseURL string) mcp.Credentials {
	return mcp.Credentials{
		"base_url":                    baseURL,
		remotemcp.CredKeyAccessToken:  "expiring-token",
		remotemcp.CredKeyRefreshToken: "refresh-0",
		remotemcp.CredKeyTokenExpiry:  time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339),
		remotemcp.CredKeyClientID:     "client-1",
	}
}

func TestExecute_RenewsTokenNearExpiry(t *testing.T) {
	fixture := newHostedFixture(t, nil)
	withTokenEndpoint(fixture, false)
	creds := expiringLogin(fixture.server.URL)
	store := &sharedStore{creds: maps.Clone(creds)}
	integration := newIntegration(t)
	SetConfigService(integration, store)
	require.NoError(t, integration.Configure(t.Context(), creds))

	result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)

	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Len(t, fixture.snapshot(), 1)
	assert.Equal(t, fixtureToken, store.creds[remotemcp.CredKeyAccessToken])
	assert.Equal(t, "refresh-1", store.creds[remotemcp.CredKeyRefreshToken])

	// The renewed login is in use: the next call needs no token request.
	fixture.oauth = nil
	result, err = integration.Execute(t.Context(), "higgsfield_balance", nil)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
}

func TestExecute_RenewalFailure(t *testing.T) {
	t.Run("expired token gives a clear error", func(t *testing.T) {
		fixture := newHostedFixture(t, nil)
		withTokenEndpoint(fixture, true)
		creds := expiringLogin(fixture.server.URL)
		creds[remotemcp.CredKeyTokenExpiry] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		integration := newIntegration(t)
		SetConfigService(integration, &sharedStore{creds: maps.Clone(creds)})
		require.NoError(t, integration.Configure(t.Context(), creds))

		result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)

		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, result.Data, "higgsfield: renew access token")
		assert.Contains(t, result.Data, "invalid_grant")
		assert.Empty(t, fixture.snapshot(), "an expired token is not sent")
	})
	t.Run("token still valid keeps working", func(t *testing.T) {
		fixture := newHostedFixture(t, nil)
		withTokenEndpoint(fixture, true)
		creds := expiringLogin(fixture.server.URL)
		creds[remotemcp.CredKeyAccessToken] = fixtureToken
		integration := newIntegration(t)
		SetConfigService(integration, &sharedStore{creds: maps.Clone(creds)})
		require.NoError(t, integration.Configure(t.Context(), creds))

		result, err := integration.Execute(t.Context(), "higgsfield_balance", nil)

		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		assert.Len(t, fixture.snapshot(), 1)
	})
}
