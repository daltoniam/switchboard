package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfluxDBDefaults(t *testing.T) {
	manager, _ := newTestManager(t)
	require.NoError(t, manager.Load())
	integration, ok := manager.cfg.Integrations["influxdb"]
	require.True(t, ok, "fresh installs must expose InfluxDB credential fields")
	require.False(t, integration.Enabled)
	require.Equal(t, map[string]string{"base_url": "", "api_token": "", "org_id": "", "ca_cert": ""}, map[string]string(integration.Credentials))
}
