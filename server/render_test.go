package server

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/render"
	"github.com/stretchr/testify/require"
)

func TestRenderSearchDiscoverability(t *testing.T) {
	registry := newMockRegistry()
	byIntegration := map[string][]mcp.ToolDefinition{}
	for _, entry := range collectAllTools() {
		byIntegration[entry.Integration] = append(byIntegration[entry.Integration], entry.Tool)
	}
	configuration := map[string]*mcp.IntegrationConfig{"render": {Enabled: true}}
	for name, tools := range byIntegration {
		require.NoError(t, registry.Register(&mockIntegration{name: name, tools: tools}))
		configuration[name] = &mcp.IntegrationConfig{Enabled: true}
	}
	require.NoError(t, registry.Register(render.New()))
	gateway := New(&mcp.Services{Config: newMockConfigService(configuration), Registry: registry})
	for _, scenario := range []struct {
		query string
		want  string
	}{
		{"render deploy", "render_list_deploys"},
		{"redeploy render service", "render_trigger_deploy"},
		{"render deploys", "render_list_deploys"},
		{"render logs", "render_list_logs"},
		{"restart render service", "render_trigger_deploy"},
		{"restart service", "render_trigger_deploy"},
		{"render build failed", "render_list_deploys"},
		{"render workspaces", "render_list_workspaces"},
		{"render services", "render_list_services"},
		{"render service crashed", "render_list_events"},
		{"render env vars", "render_update_environment_variables"},
		{"render postgres database", "render_list_postgres_instances"},
		{"render cpu memory metrics", "render_get_metrics"},
		{"render cron job", "render_run_cron_job"},
	} {
		t.Run(scenario.query, func(t *testing.T) {
			var first []string
			for attempt := range 2 {
				gateway.RefreshSearchIndex()
				result, err := gateway.handleSearch(t.Context(), searchRequest(map[string]any{"query": scenario.query, "limit": 5}))
				require.NoError(t, err)
				require.False(t, result.IsError)
				names := searchToolNames(t, parseSearchResponse(t, result))
				require.Contains(t, names, scenario.want, "%q top 5: %v", scenario.query, names)
				if attempt == 0 {
					first = names
				} else {
					require.Equal(t, first, names)
				}
			}
		})
	}
}

func TestRenderDiscoveryVisibility(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		enabled     bool
		discoverAll bool
		wantTotal   int
	}{
		{"enabled", true, false, 48},
		{"disabled", false, false, 0},
		{"disabled discover all", false, true, 48},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			registry := newMockRegistry()
			require.NoError(t, registry.Register(render.New()))
			configuration := newMockConfigService(map[string]*mcp.IntegrationConfig{"render": {Enabled: scenario.enabled}})
			gateway := New(&mcp.Services{Config: configuration, Registry: registry}, WithDiscoverAll(scenario.discoverAll))
			gateway.RefreshSearchIndex()
			result, err := gateway.handleSearch(t.Context(), searchRequest(map[string]any{"integration": "render", "limit": 5}))
			require.NoError(t, err)
			require.False(t, result.IsError)
			require.Equal(t, scenario.wantTotal, parseSearchResponse(t, result).Total)
		})
	}
}
