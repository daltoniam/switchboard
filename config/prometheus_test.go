package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrometheusDefaultsAndEnvironment(t *testing.T) {
	cfg := defaultConfig()
	integration, ok := cfg.Integrations["prometheus"]
	require.True(t, ok)
	require.False(t, integration.Enabled)
	keys := []string{"base_url", "username", "password", "bearer_token", "headers", "ca_cert"}
	require.Len(t, integration.Credentials, len(keys))
	for _, key := range keys {
		require.Contains(t, integration.Credentials, key)
		require.NotEmpty(t, envMapping["prometheus"][key])
	}
}
