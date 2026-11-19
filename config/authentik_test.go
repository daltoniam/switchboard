package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthentikDefaultsAndEnvironment(t *testing.T) {
	integration := defaultConfig().Integrations["authentik"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	require.Len(t, integration.Credentials, 3)
	for _, key := range []string{"base_url", "token", "ca_cert"} {
		require.Contains(t, integration.Credentials, key)
		require.Empty(t, integration.Credentials[key])
	}
	require.Equal(t, map[string]string{"base_url": "AUTHENTIK_BASE_URL", "token": "AUTHENTIK_TOKEN", "ca_cert": "AUTHENTIK_CA_CERT"}, envMapping["authentik"])
}
