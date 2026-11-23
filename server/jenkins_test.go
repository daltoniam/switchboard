package server

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/jenkins"
	"github.com/stretchr/testify/require"
)

func TestJenkinsSearchDiscoverability(t *testing.T) {
	registry := newMockRegistry()
	byIntegration := map[string][]mcp.ToolDefinition{}
	for _, entry := range collectAllTools() {
		byIntegration[entry.Integration] = append(byIntegration[entry.Integration], entry.Tool)
	}
	configuration := map[string]*mcp.IntegrationConfig{"jenkins": {Enabled: true}}
	for name, tools := range byIntegration {
		require.NoError(t, registry.Register(&mockIntegration{name: name, tools: tools}))
		configuration[name] = &mcp.IntegrationConfig{Enabled: true}
	}
	require.NoError(t, registry.Register(jenkins.New()))
	gateway := New(&mcp.Services{Config: newMockConfigService(configuration), Registry: registry})
	for _, scenario := range []struct {
		query string
		want  string
	}{
		{"Jenkins jobs", "jenkins_list_jobs"},
		{"Jenkins folder", "jenkins_list_jobs"},
		{"Jenkins recent builds", "jenkins_get_job"},
		{"Jenkins build result", "jenkins_get_build"},
	} {
		t.Run(scenario.query, func(t *testing.T) {
			var first []string
			for attempt := range 2 {
				gateway.RefreshSearchIndex()
				result, err := gateway.handleSearch(t.Context(), searchRequest(map[string]any{"query": scenario.query, "limit": 3}))
				require.NoError(t, err)
				require.False(t, result.IsError)
				catalog := parseSearchResponse(t, result)
				names := searchToolNames(t, catalog)
				require.Contains(t, names, scenario.want)
				if attempt == 0 {
					first = names
					t.Logf("%q top 3: %v", scenario.query, names)
				} else {
					require.Equal(t, first, names)
				}
			}
		})
	}
}

func TestJenkinsDiscoveryVisibility(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		enabled     bool
		discoverAll bool
		wantTotal   int
	}{
		{"enabled", true, false, 3},
		{"disabled", false, false, 0},
		{"enabled discover all", true, true, 3},
		{"disabled discover all", false, true, 3},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			registry := newMockRegistry()
			require.NoError(t, registry.Register(jenkins.New()))
			configuration := newMockConfigService(map[string]*mcp.IntegrationConfig{"jenkins": {Enabled: scenario.enabled}})
			gateway := New(&mcp.Services{Config: configuration, Registry: registry}, WithDiscoverAll(scenario.discoverAll))
			for range 2 {
				gateway.RefreshSearchIndex()
				result, err := gateway.handleSearch(t.Context(), searchRequest(map[string]any{"integration": "jenkins", "limit": 5}))
				require.NoError(t, err)
				require.False(t, result.IsError)
				catalog := parseSearchResponse(t, result)
				require.Equal(t, scenario.wantTotal, catalog.Total)
				require.Equal(t, scenario.wantTotal, searchToolCount(t, catalog))
				if scenario.wantTotal > 0 {
					require.ElementsMatch(t, []string{"jenkins_list_jobs", "jenkins_get_job", "jenkins_get_build"}, searchToolNames(t, catalog))
				}
			}
		})
	}
}
