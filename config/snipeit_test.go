package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestSnipeITDefaults(t *testing.T) {
	integration, ok := defaultConfig().Integrations["snipeit"]
	require.True(t, ok)
	require.False(t, integration.Enabled)
	require.Equal(t, mcp.Credentials{"base_url": "", "token": "", "ca_cert": ""}, integration.Credentials)
	require.Equal(t, map[string]string{"base_url": "SNIPEIT_BASE_URL", "token": "SNIPEIT_TOKEN", "ca_cert": "SNIPEIT_CA_CERT"}, envMapping["snipeit"])
}
