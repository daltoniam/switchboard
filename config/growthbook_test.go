package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrowthBookDefaultsAndEnvironment(t *testing.T) {
	integration := defaultConfig().Integrations["growthbook"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	require.Len(t, integration.Credentials, 3)
	for _, key := range []string{"api_key", "base_url", "ca_cert"} {
		require.Contains(t, integration.Credentials, key)
		require.Empty(t, integration.Credentials[key])
	}
	require.Equal(t, map[string]string{"api_key": "GROWTHBOOK_API_KEY", "base_url": "GROWTHBOOK_BASE_URL", "ca_cert": "GROWTHBOOK_CA_CERT"}, envMapping["growthbook"])
}
