package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHiggsfieldDefaults(t *testing.T) {
	manager, _ := newTestManager(t)
	require.NoError(t, manager.Load())
	ic, ok := manager.GetIntegration("higgsfield")
	require.True(t, ok)
	assert.False(t, ic.Enabled)
	assert.Equal(t, mcp.Credentials{"mcp_access_token": "", "base_url": "", mcp.CredKeyTokenSource: ""}, ic.Credentials)
}

func TestHiggsfieldEnvironment(t *testing.T) {
	manager, _ := newTestManager(t)
	manager.envLookup = func(key string) string {
		return map[string]string{
			"HIGGSFIELD_MCP_ACCESS_TOKEN": "higgsfield-token",
			"HIGGSFIELD_MCP_BASE_URL":     "https://mcp.higgsfield.ai/mcp",
		}[key]
	}
	require.NoError(t, manager.Load())
	ic, ok := manager.GetIntegration("higgsfield")
	require.True(t, ok)
	assert.False(t, ic.Enabled, "an env token must not enable the integration by itself")
	assert.Equal(t, "higgsfield-token", ic.Credentials["mcp_access_token"])
	assert.Equal(t, "https://mcp.higgsfield.ai/mcp", ic.Credentials["base_url"])
	assert.Empty(t, manager.persisted.Integrations["higgsfield"].Credentials["mcp_access_token"], "env secrets are never persisted")
}
