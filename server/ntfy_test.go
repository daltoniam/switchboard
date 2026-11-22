package server

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/ntfy"
	"github.com/stretchr/testify/require"
)

func TestNtfyDiscoveryVisibility(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		enabled     bool
		discoverAll bool
		wantTotal   int
	}{
		{"enabled", true, false, 2},
		{"disabled", false, false, 0},
		{"enabled discover all", true, true, 2},
		{"disabled discover all", false, true, 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			registry := newMockRegistry()
			require.NoError(t, registry.Register(ntfy.New()))
			configuration := newMockConfigService(map[string]*mcp.IntegrationConfig{
				"ntfy": {Enabled: scenario.enabled},
			})
			gateway := New(&mcp.Services{Config: configuration, Registry: registry}, WithDiscoverAll(scenario.discoverAll))
			for range 2 {
				gateway.RefreshSearchIndex()
				result, err := gateway.handleSearch(t.Context(), searchRequest(map[string]any{"integration": "ntfy", "limit": 5}))
				require.NoError(t, err)
				require.False(t, result.IsError)
				catalog := parseSearchResponse(t, result)
				require.Equal(t, scenario.wantTotal, catalog.Total)
				require.Equal(t, scenario.wantTotal, searchToolCount(t, catalog))
				if scenario.wantTotal != 0 {
					require.ElementsMatch(t, []string{"ntfy_list_messages", "ntfy_get_message"}, searchToolNames(t, catalog))
				}
			}
		})
	}
}
