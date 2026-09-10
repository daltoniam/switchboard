package wasm

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schemaConfigBarrier struct {
	mcp.ConfigService
	once   sync.Once
	rotate func()
}

func (s *schemaConfigBarrier) GetIntegration(name string) (*mcp.IntegrationConfig, bool) {
	ic, ok := s.ConfigService.GetIntegration(name)
	s.once.Do(s.rotate)
	return ic, ok
}

func (s *schemaConfigBarrier) UpdateIntegration(name string, update func(*mcp.IntegrationConfig) error) error {
	return s.ConfigService.(mcp.IntegrationConfigUpdater).UpdateIntegration(name, update)
}

func TestLoadPluginSchemaMergePreservesFreshCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, store.SetIntegration("example", &mcp.IntegrationConfig{Credentials: mcp.Credentials{"oauth_refresh_token": "before"}}))
	barrier := &schemaConfigBarrier{ConfigService: store, rotate: func() {
		require.NoError(t, store.SetIntegration("example", &mcp.IntegrationConfig{
			Credentials: mcp.Credentials{"oauth_refresh_token": "after", "unrelated": "updated"},
			ToolGlobs:   []string{"example_echo"},
			Identities:  map[string]mcp.IntegrationIdentity{"work": {Metadata: map[string]string{"label": "keep"}}},
		}))
	}}
	rt, err := NewRuntime(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = rt.Close(t.Context()) })
	path := filepath.Join(t.TempDir(), "example.wasm")
	require.NoError(t, os.WriteFile(path, exampleWasm, 0600))
	loader := NewLoader(rt, newLoaderRegistry(), barrier)
	require.NoError(t, loader.LoadPlugin(t.Context(), path, ""))
	require.NoError(t, store.Load())
	saved, _ := store.GetIntegration("example")
	assert.Equal(t, "after", saved.Credentials["oauth_refresh_token"])
	assert.Equal(t, "updated", saved.Credentials["unrelated"])
	assert.Contains(t, saved.Credentials, "api_key")
	assert.Contains(t, saved.Credentials, "base_url")
	assert.Equal(t, []string{"example_echo"}, saved.ToolGlobs)
	assert.Equal(t, "keep", saved.Identities["work"].Metadata["label"])
}
