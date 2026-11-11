package config

import (
	"os"
	"path/filepath"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestKeycloakDefaultsAndEnvironment(t *testing.T) {
	manager := &manager{filePath: filepath.Join(t.TempDir(), "config.json"), envLookup: os.Getenv}
	require.NoError(t, manager.Load())
	integration, ok := manager.GetIntegration("keycloak")
	require.True(t, ok)
	require.False(t, integration.Enabled)
	require.Equal(t, mcp.Credentials{"base_url": "", "realm": "", "client_id": "", "client_secret": ""}, integration.Credentials)
	require.ElementsMatch(t, []string{"base_url", "realm", "client_id", "client_secret"}, manager.DefaultCredentialKeys("keycloak"))
	for key, value := range map[string]string{"KEYCLOAK_BASE_URL": "https://identity.example.test/auth", "KEYCLOAK_REALM": "support", "KEYCLOAK_CLIENT_ID": "reader", "KEYCLOAK_CLIENT_SECRET": "fixture-secret"} {
		t.Setenv(key, value)
	}
	require.NoError(t, manager.Load())
	integration, ok = manager.GetIntegration("keycloak")
	require.True(t, ok)
	require.Equal(t, mcp.Credentials{"base_url": "https://identity.example.test/auth", "realm": "support", "client_id": "reader", "client_secret": "fixture-secret"}, integration.Credentials)
	require.False(t, integration.Enabled)
}
