package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/panther"
	"github.com/stretchr/testify/require"
)

func TestPantherSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+50)
	for _, entry := range raw {
		if entry.Integration == "panther" {
			continue
		}
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range panther.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "panther", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, test := range []struct{ query, want string }{
		{"siem security alerts", "panther_list_alerts"},
		{"panther open critical alerts", "panther_list_alerts"},
		{"panther incident triage", "panther_list_alerts"},
		{"panther resolve alert", "panther_update_alert_status"},
		{"panther detection rules", "panther_list_rules"},
		{"panther disable rule", "panther_set_detection_enabled"},
		{"panther data lake sql query", "panther_query_data_lake"},
		{"panther threat hunting log search", "panther_query_data_lake"},
		{"panther log sources ingestion health", "panther_list_log_sources"},
		{"panther table columns schema", "panther_get_table_schema"},
		{"panther cloud security policies", "panther_list_policies"},
		{"panther api token permissions", "panther_get_permissions"},
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
