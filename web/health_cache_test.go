package web

import (
	"context"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHealthCacheServices() (*healthCache, *mockRegistry, *mockConfigService) {
	reg := newMockRegistry()
	cfgService := newMockConfigService(map[string]*mcp.IntegrationConfig{})

	reg.Register(&mockIntegration{name: "alpha", healthy: true, tools: []mcp.ToolDefinition{{Name: mcp.ToolName("alpha_list")}}})
	reg.Register(&mockIntegration{name: "beta", healthy: false, tools: []mcp.ToolDefinition{{Name: mcp.ToolName("beta_list")}}})

	cfgService.cfg.Integrations["alpha"] = &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "tok"},
	}
	cfgService.cfg.Integrations["beta"] = &mcp.IntegrationConfig{
		Enabled:     true,
		Credentials: mcp.Credentials{"token": "tok"},
	}

	services := &mcp.Services{Config: cfgService, Registry: reg}
	hc := newHealthCache(services)
	return hc, reg, cfgService
}

func TestHealthCache_RefreshAll(t *testing.T) {
	hc, _, _ := setupHealthCacheServices()

	_, ok := hc.get("alpha")
	assert.False(t, ok, "no entry before refresh")

	ctx := context.Background()
	hc.refreshAll(ctx)

	entry, ok := hc.get("alpha")
	require.True(t, ok)
	assert.True(t, entry.Healthy)
	assert.True(t, entry.Enabled)
	assert.False(t, entry.CheckedAt.IsZero())
	assert.WithinDuration(t, time.Now(), entry.CheckedAt, 5*time.Second)

	entry, ok = hc.get("beta")
	require.True(t, ok)
	assert.False(t, entry.Healthy)
}

func TestHealthCache_RefreshAll_EnablesOnHealthy(t *testing.T) {
	reg := newMockRegistry()
	cfgService := newMockConfigService(map[string]*mcp.IntegrationConfig{})

	reg.Register(&mockIntegration{name: "gamma", healthy: true})
	cfgService.cfg.Integrations["gamma"] = &mcp.IntegrationConfig{
		Enabled:     false,
		Credentials: mcp.Credentials{"token": "tok"},
	}

	services := &mcp.Services{Config: cfgService, Registry: reg}
	hc := newHealthCache(services)

	hc.refreshAll(context.Background())

	entry, ok := hc.get("gamma")
	require.True(t, ok)
	assert.True(t, entry.Enabled)

	ic, _ := cfgService.GetIntegration("gamma")
	assert.True(t, ic.Enabled, "config should be updated to enabled")
}

func TestHealthCache_RefreshAll_NoConfig(t *testing.T) {
	reg := newMockRegistry()
	cfgService := newMockConfigService(map[string]*mcp.IntegrationConfig{})

	reg.Register(&mockIntegration{name: "nocfg", healthy: true})

	services := &mcp.Services{Config: cfgService, Registry: reg}
	hc := newHealthCache(services)

	hc.refreshAll(context.Background())

	entry, ok := hc.get("nocfg")
	require.True(t, ok)
	assert.False(t, entry.Healthy)
	assert.False(t, entry.Enabled)
}

func TestHealthCache_Get_MissReturnsZero(t *testing.T) {
	hc, _, _ := setupHealthCacheServices()
	entry, ok := hc.get("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, healthEntry{}, entry)
}

func TestHealthCache_RefreshAll_UpdatesTimestamp(t *testing.T) {
	hc, _, _ := setupHealthCacheServices()

	hc.refreshAll(context.Background())
	first, _ := hc.get("alpha")

	time.Sleep(10 * time.Millisecond)
	hc.refreshAll(context.Background())
	second, _ := hc.get("alpha")

	assert.True(t, second.CheckedAt.After(first.CheckedAt))
}
