package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/nexus"
	"github.com/stretchr/testify/require"
)

func TestNexusSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+3)
	for _, entry := range raw {
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range nexus.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "nexus", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, test := range []struct{ query, want string }{
		{"nexus missing maven package version", "nexus_list_components"},
		{"nexus search private npm package group version", "nexus_search_components"},
		{"nexus get component asset checksum metadata", "nexus_get_component"},
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
