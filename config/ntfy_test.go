package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNtfyDefaultsAndEnvironment(t *testing.T) {
	configuration := defaultConfig()
	integration := configuration.Integrations["ntfy"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	require.Len(t, integration.Credentials, 4)
	for _, key := range []string{"base_url", "topic", "api_token", "ca_cert"} {
		require.Contains(t, integration.Credentials, key)
		require.Empty(t, integration.Credentials[key])
	}
	require.Equal(t, map[string]string{"base_url": "NTFY_BASE_URL", "topic": "NTFY_TOPIC", "api_token": "NTFY_API_TOKEN", "ca_cert": "NTFY_CA_CERT"}, envMapping["ntfy"])
}
