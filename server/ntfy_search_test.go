package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/ntfy"
	"github.com/stretchr/testify/require"
)

func TestNtfySearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+2)
	for _, entry := range raw {
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range ntfy.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "ntfy", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, scenario := range []struct{ query, want string }{
		{"ntfy recent private topic notifications", "ntfy_list_messages"},
		{"ntfy backup deployment alerts inbox", "ntfy_list_messages"},
		{"ntfy get exact cached message by id", "ntfy_get_message"},
	} {
		t.Run(scenario.query, func(t *testing.T) {
			results := scoreTools(scenario.query, tools, idf, synonyms)
			names := make([]string, 0, 5)
			for _, result := range results[:min(len(results), 5)] {
				names = append(names, string(result.Tool.Name))
			}
			require.Contains(t, names, scenario.want)
			t.Logf("query=%q top5=%v", scenario.query, names)
		})
	}
}
