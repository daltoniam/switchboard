package pluginoauth

import (
	"context"
	"maps"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredentialEditWaitsForRotationAndPreservesAuthoritativeTokens(t *testing.T) {
	m, p, store := setupManager(t)
	q := start(t, m, p)
	require.NoError(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	editor, ok := any(m).(interface {
		EditCredentials(context.Context, mcp.Credentials, bool) error
	})
	require.True(t, ok, "manager must serialize credential edits")
	stale := maps.Clone(store.ic.Credentials)
	m.mu.Lock()
	started, done := make(chan struct{}), make(chan error, 1)
	go func() {
		close(started)
		done <- editor.EditCredentials(t.Context(), mcp.Credentials{"base_url": "https://updated.example", "oauth_refresh_token": stale["oauth_refresh_token"]}, true)
	}()
	<-started
	m.creds["oauth_refresh_token"] = "latest-rotation"
	m.dirty, m.verified = true, true
	m.mu.Unlock()
	require.NoError(t, <-done)
	assert.Equal(t, "latest-rotation", store.ic.Credentials["oauth_refresh_token"])
	assert.Equal(t, "https://updated.example", store.ic.Credentials["base_url"])
	assert.Equal(t, []string{"primer_read_*"}, store.ic.ToolGlobs)
}

type updatingStore struct {
	*memoryStore
	before func(*mcp.IntegrationConfig)
}

func (s *updatingStore) UpdateIntegration(name string, update func(*mcp.IntegrationConfig) error) error {
	s.before(s.ic)
	ic := *s.ic
	ic.Credentials = maps.Clone(s.ic.Credentials)
	if err := update(&ic); err != nil {
		return err
	}
	return s.SetIntegration(name, &ic)
}

func TestPersistenceCannotOverwriteConcurrentSettingsChange(t *testing.T) {
	m, p, store := setupManager(t)
	store.ic.Credentials = p.credentials()
	q := start(t, m, p)
	m.store = &updatingStore{memoryStore: store, before: func(ic *mcp.IntegrationConfig) {
		ic.Credentials["oauth_issuer"] = "https://new.example"
	}}
	require.Error(t, m.Callback(t.Context(), "code", q.Get("state"), "browser-cookie", ""))
	assert.Equal(t, "https://new.example", store.ic.Credentials["oauth_issuer"])
	assert.Empty(t, store.ic.Credentials["oauth_refresh_token"])
}
