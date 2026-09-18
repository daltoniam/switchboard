package config

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotionMCPDefaultsAndIsolation(t *testing.T) {
	manager, _ := newTestManager(t)
	require.NoError(t, manager.Load())
	original, ok := manager.GetIntegration("notion")
	require.True(t, ok)
	original.Enabled = true
	original.Credentials["token_v2"] = "existing-notion-cookie"
	require.NoError(t, manager.SetIntegration("notion", original))

	remote, ok := manager.GetIntegration("notion-mcp")
	require.True(t, ok)
	assert.False(t, remote.Enabled)
	assert.Equal(t, mcp.Credentials{"mcp_access_token": "", "base_url": "", mcp.CredKeyTokenSource: ""}, remote.Credentials)
	assert.ElementsMatch(t, []string{"mcp_access_token", "base_url", "token_source"}, manager.DefaultCredentialKeys("notion-mcp"))
	remote.Enabled = true
	remote.Credentials["mcp_access_token"] = "remote-notion-token"
	require.NoError(t, manager.SetIntegration("notion-mcp", remote))
	require.NoError(t, manager.Load())
	retained, ok := manager.GetIntegration("notion")
	require.True(t, ok)
	assert.Equal(t, original, retained)
	remote, ok = manager.GetIntegration("notion-mcp")
	require.True(t, ok)
	assert.True(t, remote.Enabled)
	assert.Equal(t, "remote-notion-token", remote.Credentials["mcp_access_token"])
}

func TestNotionMCPEnvironment(t *testing.T) {
	manager, _ := newTestManager(t)
	manager.envLookup = func(key string) string {
		return map[string]string{
			"NOTION_MCP_ACCESS_TOKEN": "remote-notion-token",
			"NOTION_MCP_BASE_URL":     "https://mcp.notion.com/mcp",
		}[key]
	}
	require.NoError(t, manager.Load())
	remote, ok := manager.GetIntegration("notion-mcp")
	require.True(t, ok)
	assert.False(t, remote.Enabled)
	assert.Equal(t, "remote-notion-token", remote.Credentials["mcp_access_token"])
	assert.Equal(t, "https://mcp.notion.com/mcp", remote.Credentials["base_url"])
	original, ok := manager.GetIntegration("notion")
	require.True(t, ok)
	assert.False(t, original.Enabled)
	assert.Empty(t, original.Credentials["token_v2"])
	assert.Empty(t, manager.persisted.Integrations["notion-mcp"].Credentials["mcp_access_token"])
}
