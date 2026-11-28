package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPantherDefaults(t *testing.T) {
	manager, _ := newTestManager(t)
	require.NoError(t, manager.Load())
	ic, ok := manager.GetIntegration("panther")
	require.True(t, ok)
	assert.False(t, ic.Enabled)
	assert.Equal(t, mcp.Credentials{"instance_url": "", "api_token": "", "mcp_access_token": "", mcp.CredKeyTokenSource: ""}, ic.Credentials)
}

func TestPantherEnvironment(t *testing.T) {
	manager, _ := newTestManager(t)
	manager.envLookup = func(key string) string {
		return map[string]string{
			"PANTHER_INSTANCE_URL":     "https://acme.runpanther.net",
			"PANTHER_API_TOKEN":        "panther-api-token",
			"PANTHER_MCP_ACCESS_TOKEN": "panther-oauth-token",
		}[key]
	}
	require.NoError(t, manager.Load())
	ic, ok := manager.GetIntegration("panther")
	require.True(t, ok)
	assert.False(t, ic.Enabled, "env credentials must not enable the integration by themselves")
	assert.Equal(t, "https://acme.runpanther.net", ic.Credentials["instance_url"])
	assert.Equal(t, "panther-api-token", ic.Credentials["api_token"])
	assert.Equal(t, "panther-oauth-token", ic.Credentials["mcp_access_token"])
	assert.Empty(t, manager.persisted.Integrations["panther"].Credentials["api_token"], "env secrets are never persisted")
}
