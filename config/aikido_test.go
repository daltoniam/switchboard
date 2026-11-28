package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAikidoDefaultsAndEnvironment(t *testing.T) {
	integration := defaultConfig().Integrations["aikido"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	require.Len(t, integration.Credentials, 4)
	for _, key := range []string{"client_id", "client_secret", "region", "base_url"} {
		require.Contains(t, integration.Credentials, key)
		require.Empty(t, integration.Credentials[key])
	}
	require.Equal(t, map[string]string{
		"client_id":     "AIKIDO_CLIENT_ID",
		"client_secret": "AIKIDO_CLIENT_SECRET",
		"region":        "AIKIDO_REGION",
		"base_url":      "AIKIDO_BASE_URL",
	}, envMapping["aikido"])
}
