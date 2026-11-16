package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/snipeit"
	"github.com/stretchr/testify/require"
)

func TestSnipeITSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+3)
	for _, entry := range raw {
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range snipeit.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "snipeit", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, tc := range []struct{ query, want string }{
		{"snipeit company hardware inventory incident", "snipeit_list_assets"},
		{"Snipe-IT get hardware asset numeric ID assignment", "snipeit_get_asset"},
		{"Snipe-IT exact asset tag", "snipeit_get_asset_by_tag"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			results := scoreTools(tc.query, tools, idf, synonyms)
			names := make([]string, 0, 5)
			for _, result := range results[:min(len(results), 5)] {
				names = append(names, string(result.Tool.Name))
			}
			require.Contains(t, names, tc.want)
			t.Logf("query=%q top5=%v", tc.query, names)
		})
	}
}
