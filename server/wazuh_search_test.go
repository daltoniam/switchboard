package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/wazuh"
	"github.com/stretchr/testify/require"
)

func TestWazuhSearchDiscoverability(t *testing.T) {
	catalog := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(catalog)+3)
	for _, entry := range catalog {
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range wazuh.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "wazuh", Tool: tool})
	}
	idf, synonyms := computeIDF(tools), buildSynonymMap(synonymGroups)
	for _, test := range []struct{ query, want string }{
		{"wazuh find disconnected endpoint security agents", "wazuh_list_agents"},
		{"wazuh exact agent last check-in OS version", "wazuh_get_agent"},
		{"wazuh connection configuration synchronization summary totals", "wazuh_get_agent_status_summary"},
	} {
		t.Run(test.query, func(t *testing.T) {
			results := scoreTools(test.query, tools, idf, synonyms)
			names := make([]string, 0, 5)
			for _, result := range results[:min(len(results), 5)] {
				names = append(names, string(result.Tool.Name))
			}
			require.Contains(t, names, test.want)
			t.Logf("query=%q top5=%v", test.query, names)
		})
	}
}
