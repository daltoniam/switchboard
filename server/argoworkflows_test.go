package server

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/argoworkflows"
	"github.com/stretchr/testify/require"
)

func TestArgoWorkflowsSearchDiscovery(t *testing.T) {
	registry := newMockRegistry()
	require.NoError(t, registry.Register(argoworkflows.New()))
	server := New(&mcp.Services{
		Registry: registry,
		Config: newMockConfigService(map[string]*mcp.IntegrationConfig{
			"argoworkflows": {Enabled: true, Credentials: mcp.Credentials{"base_url": "https://argo.example.test", "namespace": "approved", "token": "fixture-token"}},
		}),
	})
	for _, query := range []string{"Argo workflow failed", "Kubernetes DAG status", "inspect workflow progress"} {
		t.Run(query, func(t *testing.T) {
			result, err := server.handleSearch(t.Context(), searchRequest(map[string]any{"query": query, "limit": 3}))
			require.NoError(t, err)
			require.False(t, result.IsError)
			response := parseSearchResponse(t, result)
			names := searchToolNames(t, response)
			require.Contains(t, names, "argoworkflows_list_workflows")
			require.Contains(t, names, "argoworkflows_get_workflow")
			t.Logf("%q: %v", query, names)
		})
	}
}
