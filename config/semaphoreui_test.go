package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestSemaphoreUIDefaults(t *testing.T) {
	integration, ok := defaultConfig().Integrations["semaphoreui"]
	require.True(t, ok)
	require.False(t, integration.Enabled)
	require.Equal(t, mcp.Credentials{"base_url": "", "api_token": "", "project_id": "", "ca_cert": ""}, integration.Credentials)
	require.Equal(t, map[string]string{"base_url": "SEMAPHOREUI_BASE_URL", "api_token": "SEMAPHOREUI_API_TOKEN", "project_id": "SEMAPHOREUI_PROJECT_ID", "ca_cert": "SEMAPHOREUI_CA_CERT"}, envMapping["semaphoreui"])
}
