package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/gatus"
	"github.com/stretchr/testify/require"
)

func TestGatusSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+3)
	for _, entry := range raw {
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range gatus.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "gatus", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, tc := range []struct{ query, want string }{
		{"Gatus find down unhealthy monitored endpoints", "gatus_list_endpoints"},
		{"Gatus endpoint probe result history", "gatus_get_endpoint"},
		{"Gatus endpoint uptime ratio last 24h", "gatus_get_uptime"},
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
