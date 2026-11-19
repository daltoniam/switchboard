package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/growthbook"
	"github.com/stretchr/testify/require"
)

func TestGrowthBookSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+30)
	for _, entry := range raw {
		if entry.Integration == "growthbook" {
			continue
		}
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range growthbook.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "growthbook", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, test := range []struct{ query, want string }{
		{"growthbook feature flags", "growthbook_list_features"},
		{"is growthbook flag enabled in production", "growthbook_list_features"},
		{"growthbook a/b test experiments", "growthbook_list_experiments"},
		{"growthbook experiment results winner chance to beat control", "growthbook_get_experiment_results"},
		{"growthbook turn off flag kill switch", "growthbook_toggle_feature"},
		{"growthbook create feature flag", "growthbook_create_feature"},
		{"growthbook stale flags cleanup", "growthbook_get_feature_staleness"},
		{"growthbook who changed flag audit log", "growthbook_list_audit_logs"},
		{"growthbook fact metrics", "growthbook_list_fact_metrics"},
		{"growthbook saved group audience", "growthbook_list_saved_groups"},
		{"growthbook refresh experiment results", "growthbook_refresh_experiment_results"},
		{"growthbook flag code references", "growthbook_get_feature_code_refs"},
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
