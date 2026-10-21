package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/mysql"
	"github.com/stretchr/testify/require"
)

func TestMySQLSearchDiscovery(t *testing.T) {
	var tools []toolWithIntegration
	for _, item := range collectAllTools() {
		tools = append(tools, toolWithIntegration{Integration: item.Integration, Tool: item.Tool})
	}
	for _, tool := range mysql.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "mysql", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	cases := []struct{ query, expected string }{
		{"MySQL databases schema discovery", "mysql_list_databases"},
		{"MySQL table columns data types", "mysql_list_columns"},
		{"MySQL indexes cardinality", "mysql_list_indexes"},
		{"MySQL foreign key relationships", "mysql_list_foreign_keys"},
		{"MySQL running queries processlist", "mysql_running_queries"},
		{"MySQL explain query plans", "mysql_explain"},
	}
	for _, test := range cases {
		t.Run(test.query, func(t *testing.T) {
			results := scoreTools(test.query, tools, idf, synonyms)
			var top []string
			for index, result := range results {
				if index == 3 {
					break
				}
				top = append(top, string(result.Tool.Name))
			}
			require.Contains(t, top, test.expected)
			t.Logf("top 3: %v", top)
		})
	}
}
