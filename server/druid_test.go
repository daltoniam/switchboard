package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/druid"
	"github.com/stretchr/testify/require"
)

func TestDruidSearchDiscoverability(t *testing.T) {
	tools := []toolWithIntegration{}
	for _, item := range collectAllTools() {
		tools = append(tools, toolWithIntegration{Integration: item.Integration, Tool: item.Tool})
	}
	for _, tool := range druid.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "druid", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, scenario := range []struct{ query, expected string }{
		{"druid ingestion diagnosis", "druid_list_datasources"},
		{"druid datasource segment summary", "druid_get_datasource"},
		{"druid segment load percentages", "druid_get_load_status"},
		{"druid failed ingestion tasks", "druid_list_tasks"},
		{"druid task status failure context", "druid_get_task_status"},
	} {
		t.Run(scenario.query, func(t *testing.T) {
			results := scoreTools(scenario.query, tools, idf, synonyms)
			names := []string{}
			for _, result := range results[:min(3, len(results))] {
				names = append(names, string(result.Tool.Name))
			}
			require.Contains(t, names, scenario.expected, "expected tool in top 3 across all benchmark adapters")
		})
	}
}
