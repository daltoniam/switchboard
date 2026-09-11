package config

import (
	"errors"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateConfigUsesFreshCopy(t *testing.T) {
	m, _ := newTestManager(t)
	require.NoError(t, m.Load())
	require.NoError(t, m.SetIntegration("primer", &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"oauth_refresh_token": "before"}}))
	updater, ok := any(m).(interface {
		UpdateConfig(func(*mcp.Config) error) error
	})
	require.True(t, ok, "whole-config edits must support atomic fresh-copy updates")
	stale := m.Get()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		close(entered)
		<-release
		done <- updater.UpdateConfig(func(cfg *mcp.Config) error {
			cfg.SessionStore = "file"
			cfg.Marketplace = &mcp.MarketplaceConfig{AutoUpdate: true}
			return nil
		})
	}()
	<-entered
	err := m.UpdateIntegration("primer", func(ic *mcp.IntegrationConfig) error {
		ic.Credentials["oauth_refresh_token"] = "after"
		ic.ToolGlobs = []string{"primer_*"}
		return nil
	})
	close(release)
	require.NoError(t, err)
	require.NoError(t, <-done)
	require.NoError(t, m.Load())
	got := m.Get()
	assert.Equal(t, "after", got.Integrations["primer"].Credentials["oauth_refresh_token"])
	assert.Equal(t, []string{"primer_*"}, got.Integrations["primer"].ToolGlobs)
	assert.Equal(t, "file", got.SessionStore)
	assert.True(t, got.Marketplace.AutoUpdate)
	assert.Equal(t, "before", stale.Integrations["primer"].Credentials["oauth_refresh_token"])
	before := m.Get()
	require.Error(t, updater.UpdateConfig(func(cfg *mcp.Config) error {
		cfg.Integrations["primer"].Credentials["oauth_refresh_token"] = "discard"
		return errors.New("cancel edit")
	}))
	assert.Equal(t, before, m.Get())
}
