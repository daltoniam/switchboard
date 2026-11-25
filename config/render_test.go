package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderDefaultsAndEnvironment(t *testing.T) {
	integration := defaultConfig().Integrations["render"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	require.Len(t, integration.Credentials, 3)
	for _, key := range []string{"api_key", "workspace_id", "base_url"} {
		require.Contains(t, integration.Credentials, key)
		require.Empty(t, integration.Credentials[key])
	}
	require.Equal(t, map[string]string{"api_key": "RENDER_API_KEY", "workspace_id": "RENDER_WORKSPACE_ID", "base_url": "RENDER_MCP_BASE_URL"}, envMapping["render"])

	manager, _ := newTestManager(t)
	values := map[string]string{"RENDER_API_KEY": "rnd_test", "RENDER_WORKSPACE_ID": "tea-1", "RENDER_MCP_BASE_URL": "https://mcp.example.invalid"}
	manager.envLookup = func(key string) string { return values[key] }
	require.NoError(t, manager.Load())
	loaded, ok := manager.GetIntegration("render")
	require.True(t, ok)
	for key, variable := range envMapping["render"] {
		require.Equal(t, values[variable], loaded.Credentials[key])
	}
}
