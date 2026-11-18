package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatusDefaultsAndEnvironment(t *testing.T) {
	configuration := defaultConfig()
	integration := configuration.Integrations["gatus"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	require.Len(t, integration.Credentials, 4)
	for _, key := range []string{"base_url", "username", "password", "ca_cert"} {
		require.Contains(t, integration.Credentials, key)
		require.Empty(t, integration.Credentials[key])
	}
	require.Equal(t, map[string]string{"base_url": "GATUS_BASE_URL", "username": "GATUS_USERNAME", "password": "GATUS_PASSWORD", "ca_cert": "GATUS_CA_CERT"}, envMapping["gatus"])
}
