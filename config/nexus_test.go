package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestNexusDefaults(t *testing.T) {
	integration, ok := defaultConfig().Integrations["nexus"]
	require.True(t, ok)
	require.False(t, integration.Enabled)
	require.Equal(t, mcp.Credentials{"base_url": "", "username": "", "password": "", "repositories": "", "ca_cert": ""}, integration.Credentials)
	require.Equal(t, map[string]string{"base_url": "NEXUS_BASE_URL", "username": "NEXUS_USERNAME", "password": "NEXUS_PASSWORD", "repositories": "NEXUS_REPOSITORIES", "ca_cert": "NEXUS_CA_CERT"}, envMapping["nexus"])
}
