package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestWazuhDefaults(t *testing.T) {
	integration, ok := defaultConfig().Integrations["wazuh"]
	require.True(t, ok)
	require.False(t, integration.Enabled)
	require.Equal(t, mcp.Credentials{"base_url": "", "username": "", "password": "", "ca_cert": ""}, integration.Credentials)
	require.Equal(t, map[string]string{"base_url": "WAZUH_BASE_URL", "username": "WAZUH_USERNAME", "password": "WAZUH_PASSWORD", "ca_cert": "WAZUH_CA_CERT"}, envMapping["wazuh"])
}
