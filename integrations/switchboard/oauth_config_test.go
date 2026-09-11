package switchboard

import (
	"context"
	"errors"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	"github.com/daltoniam/switchboard/pluginoauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type credentialEditingIntegration struct {
	fakeIntegration
	edit func(context.Context, mcp.Credentials, bool) error
}

func (i *credentialEditingIntegration) EditCredentials(ctx context.Context, updates mcp.Credentials, enabled bool) error {
	return i.edit(ctx, updates, enabled)
}

func TestConfigureIntegrationUsesCredentialEditor(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			store, err := config.NewManager()
			require.NoError(t, err)
			require.NoError(t, store.SetIntegration("primer", &mcp.IntegrationConfig{
				Enabled:     true,
				Credentials: mcp.Credentials{"oauth_refresh_token": "rotated", "base_url": "original"},
				ToolGlobs:   []string{"primer_*"},
				Identities:  map[string]mcp.IntegrationIdentity{"existing": {Metadata: map[string]string{"label": "keep"}}},
			}))
			services := newTestServices()
			services.Config = store
			called := false
			integration := &credentialEditingIntegration{fakeIntegration: fakeIntegration{name: "primer", configErr: errors.New("must not configure a stale snapshot")}}
			integration.edit = func(ctx context.Context, updates mcp.Credentials, gotEnabled bool) error {
				called = true
				assert.Equal(t, enabled, gotEnabled)
				assert.Equal(t, mcp.Credentials{"base_url": "updated", "oauth_refresh_token": "stale"}, updates)
				return pluginoauth.New("primer", store).EditCredentials(ctx, updates, gotEnabled)
			}
			require.NoError(t, services.Registry.Register(integration))
			result, err := configureIntegration(t.Context(), &switchboardInt{services: services}, map[string]any{
				"name": "primer", "enabled": enabled,
				"credentials": map[string]any{"base_url": "updated", "oauth_refresh_token": "stale"},
				"identities":  map[string]any{"new": map[string]any{"metadata": map[string]any{"label": "added"}}},
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.True(t, called)
			require.NoError(t, store.Load())
			saved, _ := store.GetIntegration("primer")
			assert.Equal(t, "rotated", saved.Credentials["oauth_refresh_token"])
			assert.Equal(t, "updated", saved.Credentials["base_url"])
			assert.Equal(t, enabled, saved.Enabled)
			assert.Equal(t, []string{"primer_*"}, saved.ToolGlobs)
			assert.Equal(t, "keep", saved.Identities["existing"].Metadata["label"])
			assert.Equal(t, "added", saved.Identities["new"].Metadata["label"])
		})
	}
}

func TestConfigureIntegrationCredentialEditorFailure(t *testing.T) {
	services := newTestServices()
	integration := &credentialEditingIntegration{fakeIntegration: fakeIntegration{name: "primer"}, edit: func(context.Context, mcp.Credentials, bool) error {
		return pluginoauth.ErrPersistence
	}}
	require.NoError(t, services.Registry.Register(integration))
	result, err := configureIntegration(t.Context(), &switchboardInt{services: services}, map[string]any{"name": "primer"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	_, exists := services.Config.GetIntegration("primer")
	assert.False(t, exists)
}
