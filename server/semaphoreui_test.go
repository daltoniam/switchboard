package server

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/semaphoreui"
	"github.com/stretchr/testify/require"
)

func TestHandleSearch_SemaphoreUIDiscovery(t *testing.T) {
	registry := newMockRegistry()
	require.NoError(t, registry.Register(semaphoreui.New()))
	server := New(&mcp.Services{
		Registry: registry,
		Config:   newMockConfigService(map[string]*mcp.IntegrationConfig{"semaphoreui": {Enabled: true, Credentials: mcp.Credentials{"api_token": "catalog-only"}}}),
	})
	for _, test := range []struct{ query, want string }{
		{"Semaphore UI project", "semaphoreui_get_project"},
		{"Semaphore Ansible automation templates", "semaphoreui_list_templates"},
		{"Semaphore automation tasks status", "semaphoreui_list_tasks"},
		{"Semaphore exact template", "semaphoreui_get_template"},
		{"Semaphore exact task status", "semaphoreui_get_task"},
	} {
		t.Run(test.query, func(t *testing.T) {
			result, err := server.handleSearch(t.Context(), searchRequest(map[string]any{"query": test.query, "limit": 3}))
			require.NoError(t, err)
			require.False(t, result.IsError)
			names := searchToolNames(t, parseSearchResponse(t, result))
			require.Contains(t, names, test.want)
			t.Logf("%q top 3: %v", test.query, names)
		})
	}
	config, ok := server.services.Config.GetIntegration("semaphoreui")
	require.True(t, ok)
	config.Enabled = false
	require.NoError(t, server.services.Config.SetIntegration("semaphoreui", config))
	server.RefreshSearchIndex()
	result, err := server.handleSearch(t.Context(), searchRequest(map[string]any{"integration": "semaphoreui", "limit": 10}))
	require.NoError(t, err)
	require.Zero(t, parseSearchResponse(t, result).Total)
}
