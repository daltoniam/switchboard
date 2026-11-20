package grafana

import (
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

// Every tool that returns data has a spec, including grafana_query_datasource,
// which is a write only because the data source may run raw SQL with write
// privileges. A save returns its full confirmation.
func TestFieldCompactionSpecs_EveryReadToolHasSpec(t *testing.T) {
	g := &grafana{}
	for _, tool := range tools {
		_, ok := g.CompactSpec(tool.Name)
		if tool.Name == "grafana_save_dashboard" {
			assert.False(t, ok, "%s must return its full confirmation", tool.Name)
		} else {
			assert.True(t, ok, "%s returns data and needs a compaction spec", tool.Name)
		}
	}
}

// Shape parity: each spec, applied to a payload shaped like the handler's
// output (captured from Grafana 13.3), keeps the routing fields and drops
// the noise.
func TestFieldCompactionSpecs_Shapes(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		payload string
		keep    []string
		drop    []string
	}{
		{"grafana_search_dashboards", `[{"id":622268405960704,"uid":"svc","orgId":1,"title":"Service Overview","uri":"db/service-overview","url":"/d/svc/service-overview","slug":"","type":"dash-db","tags":["svc"],"isStarred":false,"folderId":622247308582912,"folderUid":"ops","folderTitle":"Ops","folderUrl":"/dashboards/f/ops/ops","sortMeta":0,"isDeleted":false}]`,
			[]string{`"uid":"svc"`, `"title":"Service Overview"`, `"url":"/d/svc/service-overview"`, `"folderTitle":"Ops"`}, []string{"orgId", "uri", "sortMeta", "folderId"}},
		{"grafana_get_dashboard", `{"meta":{"type":"db","canSave":true,"canAdmin":true,"slug":"s","url":"/d/svc/s","version":3,"folderUid":"ops","folderTitle":"Ops","provisioned":false,"annotationsPermissions":{"dashboard":{"canAdd":true}}},"dashboard":{"id":7,"uid":"svc","title":"S","panels":[{"type":"timeseries","targets":[{"expr":"up"}]}],"version":4}}`,
			[]string{`"panels"`, `"expr":"up"`, `"folderUid":"ops"`, `"version":4`}, []string{"canAdmin", "annotationsPermissions", "slug"}},
		{"grafana_list_folders", `[{"id":-1,"uid":"sharedwithme","title":"Shared with me"},{"id":3,"uid":"ops","title":"Ops","parentUid":"root"}]`,
			[]string{`"uid":"ops"`, `"title":"Ops"`, `"parentUid":"root"`}, []string{`"id"`}},
		{"grafana_list_datasources", `[{"id":1,"uid":"prom","orgId":1,"name":"Prometheus","type":"prometheus","typeName":"Prometheus","typeLogoUrl":"public/x.svg","access":"proxy","url":"https://prom","user":"","basicAuth":false,"isDefault":true,"jsonData":{"httpMethod":"POST"},"readOnly":false}]`,
			[]string{`"uid":"prom"`, `"name":"Prometheus"`, `"url":"https://prom"`, `"isDefault":true`}, []string{"typeLogoUrl", "jsonData", "orgId"}},
		{"grafana_query_datasource", `[{"ref_id":"A","status":200,"frames":[{"schema":{"refId":"A","meta":{"type":"timeseries-multi"},"fields":[{"name":"time","type":"time"},{"name":"A-series","type":"number","labels":{"job":"api"}}]},"data":{"values":[[1794715442915],[85.8]]}}]}]`,
			[]string{`"ref_id":"A"`, `"job":"api"`, `1794715442915`, `85.8`}, nil},
		{"grafana_get_health", `{"grafana":{"database":"ok","version":"13.3.1","commit":"abc"},"organization":{"id":1,"name":"Main Org.","address":{"city":""}}}`,
			[]string{`"version":"13.3.1"`, `"name":"Main Org."`}, []string{"address"}},
	}
	g := &grafana{}
	for _, tc := range tests {
		t.Run(string(tc.tool), func(t *testing.T) {
			fields, ok := g.CompactSpec(tc.tool)
			require.True(t, ok)
			var payload any
			require.NoError(t, json.Unmarshal([]byte(tc.payload), &payload))
			out, err := json.Marshal(mcp.CompactAny(payload, fields))
			require.NoError(t, err)
			for _, want := range tc.keep {
				assert.Contains(t, string(out), want)
			}
			for _, unwanted := range tc.drop {
				assert.NotContains(t, string(out), unwanted)
			}
		})
	}
}
