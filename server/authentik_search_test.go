package server

import (
	"testing"

	"github.com/daltoniam/switchboard/integrations/authentik"
	"github.com/stretchr/testify/require"
)

func TestAuthentikSearchDiscoverability(t *testing.T) {
	raw := collectAllTools()
	tools := make([]toolWithIntegration, 0, len(raw)+3)
	for _, entry := range raw {
		tools = append(tools, toolWithIntegration{Integration: entry.Integration, Tool: entry.Tool})
	}
	for _, tool := range authentik.New().Tools() {
		tools = append(tools, toolWithIntegration{Integration: "authentik", Tool: tool})
	}
	idf := computeIDF(tools)
	synonyms := buildSynonymMap(synonymGroups)
	for _, test := range []struct{ query, want string }{
		{"authentik resolve user identity active group", "authentik_list_users"},
		{"authentik get user identity metadata", "authentik_get_user"},
		{"authentik applications accessible exact user policy access", "authentik_list_user_applications"},
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
