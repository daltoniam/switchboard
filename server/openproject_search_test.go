package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/openproject"
)

func TestOpenProjectSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	corpus := make([]toolWithIntegration, 0, len(raw)+2)
	for _, entry := range raw {
		corpus = append(corpus, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range openproject.New().Tools() {
		corpus = append(corpus, toolWithIntegration{Integration: "openproject", Tool: tool})
	}
	idf := computeIDF(corpus)
	synonyms := buildSynonymMap(synonymGroups)
	for _, scenario := range []struct {
		query string
		tool  string
	}{
		{"find OpenProject work packages", "openproject_list_work_packages"},
		{"search OpenProject Community tasks", "openproject_list_work_packages"},
		{"get exact OpenProject work package description", "openproject_get_work_package"},
		{"OpenProject project issue status", "openproject_list_work_packages"},
	} {
		t.Run(scenario.query, func(t *testing.T) {
			results := scoreTools(scenario.query, corpus, idf, synonyms)
			for rank, result := range results[:min(3, len(results))] {
				if string(result.Tool.Name) == scenario.tool {
					t.Logf("%q found %s at rank %d across %d tools", scenario.query, scenario.tool, rank+1, len(corpus))
					return
				}
			}
			t.Fatalf("%s missing from top three for %q", scenario.tool, scenario.query)
		})
	}
}
