package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAshbyDefaults(t *testing.T) {
	manager, _ := newTestManager(t)
	require.NoError(t, manager.Load())
	ic, ok := manager.GetIntegration("ashby")
	require.True(t, ok)
	assert.False(t, ic.Enabled)
	assert.Equal(t, mcp.Credentials{"mcp_access_token": "", "base_url": "", mcp.CredKeyTokenSource: ""}, ic.Credentials)
}

func TestAshbyEnvironment(t *testing.T) {
	manager, _ := newTestManager(t)
	manager.envLookup = func(key string) string {
		return map[string]string{
			"ASHBY_MCP_ACCESS_TOKEN": "ashby-token",
			"ASHBY_MCP_BASE_URL":     "https://mcp.ashbyhq.com/mcp/v1",
		}[key]
	}
	require.NoError(t, manager.Load())
	ic, ok := manager.GetIntegration("ashby")
	require.True(t, ok)
	assert.False(t, ic.Enabled, "an env token must not enable the integration by itself")
	assert.Equal(t, "ashby-token", ic.Credentials["mcp_access_token"])
	assert.Equal(t, "https://mcp.ashbyhq.com/mcp/v1", ic.Credentials["base_url"])
	assert.Empty(t, manager.persisted.Integrations["ashby"].Credentials["mcp_access_token"], "env secrets are never persisted")
}
