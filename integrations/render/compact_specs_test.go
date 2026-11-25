package render

import (
	"encoding/json"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// uncompactedReads are read tools whose upstream output is already compact
// or has no fixed shape to whitelist.
var uncompactedReads = map[mcp.ToolName]string{
	"render_get_metrics":              "time series of timestamp/value points",
	"render_query_postgres":           "arbitrary SQL rows",
	"render_list_log_label_values":    "a list of strings",
	"render_list_workflows":           "upstream returns curated summaries",
	"render_get_workflow":             "upstream returns curated summaries",
	"render_list_workflow_versions":   "upstream returns curated summaries",
	"render_get_workflow_version":     "upstream returns curated summaries",
	"render_list_workflow_tasks":      "upstream returns curated summaries",
	"render_get_workflow_task":        "upstream returns curated summaries",
	"render_list_workflow_runs":       "upstream trims inputs, results and long errors",
	"render_get_workflow_run":         "upstream trims inputs, results and long errors",
	"render_get_workflow_run_results": "paged result text the caller concatenates",
	"render_list_flows":               "upstream returns curated summaries",
	"render_get_flow":                 "upstream returns curated summaries",
	"render_list_flow_runs":           "upstream trims inputs, results and long errors",
	"render_get_flow_run":             "upstream trims inputs, results and long errors",
	"render_get_flow_run_results":     "paged result text the caller concatenates",
	"render_list_flow_run_steps":      "upstream trims inputs, results and long errors",
	"render_get_flow_run_step":        "upstream trims inputs, results and long errors",
}

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
		assert.True(t, ok, "field compaction spec for %q has no dispatch entry", toolName)
	}
}

func TestFieldCompactionSpecs_ReadToolsCovered(t *testing.T) {
	r := &render{}
	for _, tool := range tools {
		t.Run(string(tool.Name), func(t *testing.T) {
			_, hasSpec := r.CompactSpec(tool.Name)
			_, exempt := uncompactedReads[tool.Name]
			switch {
			case tool.Access != mcp.AccessRead:
				assert.False(t, hasSpec, "mutations return their full confirmation")
				assert.False(t, exempt)
			case exempt:
				assert.False(t, hasSpec, "exempt tool has a spec; drop it from uncompactedReads")
			default:
				assert.True(t, hasSpec, "read tool needs a compaction spec or an exemption reason")
			}
		})
	}
}

// Shape parity: each spec applied to a payload shaped like the hosted
// server's output keeps routing fields and drops the noise.
func TestFieldCompactionSpecs_Shapes(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		payload string
		keep    []string
		drop    []string
	}{
		{"render_list_workspaces", `[{"id":"tea-1","name":"Team","email":"a@b.c","type":"team","ipAllowList":[{"cidrBlock":"0.0.0.0/0"}],"twoFactorAuthEnabled":true}]`,
			[]string{`"tea-1"`, `"Team"`, `"team"`}, []string{"ipAllowList", "twoFactorAuthEnabled"}},
		{"render_list_services", `[{"id":"srv-1","name":"api","type":"web_service","slug":"api","suspended":"not_suspended","autoDeploy":"yes","repo":"https://github.com/a/b","branch":"main","dashboardUrl":"https://dashboard.render.com/web/srv-1","ownerId":"tea-1","notifyOnFail":"default","rootDir":"","updatedAt":"2026-11-01T00:00:00Z","serviceDetails":{"url":"https://api.onrender.com","region":"oregon","plan":"starter","runtime":"node","numInstances":1,"openPorts":[{"port":10000}],"envSpecificDetails":{"buildCommand":"npm ci"},"healthCheckPath":"/healthz"}},{"id":"crn-1","name":"nightly","type":"cron_job","serviceDetails":{"schedule":"0 3 * * *","runtime":"python"}}]`,
			[]string{`"srv-1"`, `"crn-1"`, `"https://api.onrender.com"`, `"oregon"`, `"starter"`, `"not_suspended"`}, []string{"openPorts", "envSpecificDetails", "notifyOnFail", "healthCheckPath"}},
		{"render_get_service", `{"id":"srv-1","name":"api","type":"web_service","notifyOnFail":"default","buildFilter":{"paths":["x"]},"registryCredential":{"id":"rgc-1"},"serviceDetails":{"healthCheckPath":"/healthz","numInstances":2}}`,
			[]string{`"srv-1"`, `"/healthz"`, `"numInstances"`}, []string{"notifyOnFail", "buildFilter", "registryCredential"}},
		{"render_list_deploys", `{"items":[{"id":"dep-1","status":"build_failed","trigger":"new_commit","commit":{"id":"abc","message":"fix","createdAt":"2026-11-01T00:00:00Z"},"image":{"ref":"x","sha":"sha256:1","registryCredential":"r"},"createdAt":"2026-11-01T00:00:00Z","finishedAt":"2026-11-01T00:05:00Z","buildId":"bld-1"}],"next_cursor":"c1"}`,
			[]string{`"dep-1"`, `"build_failed"`, `"fix"`, `"c1"`}, []string{"buildId", "registryCredential", "sha256:1", `"ref"`}},
		{"render_get_deploy", `{"id":"dep-1","status":"live","commit":{"id":"abc","message":"fix"},"image":{"ref":"x","sha":"s","registryCredential":"rgc-secret"}}`,
			[]string{`"live"`, `"fix"`}, []string{"rgc-secret"}},
		{"render_get_service", `{"id":"crn-1","type":"cron_job","serviceDetails":{"schedule":"0 3 * * *","lastSuccessfulRunAt":"2026-11-01T03:00:00Z"}}`,
			[]string{`"0 3 * * *"`, `"lastSuccessfulRunAt"`}, nil},
		{"render_list_events", `{"items":[{"id":"evt-1","serviceId":"srv-1","timestamp":"2026-11-01T00:00:00Z","type":"server_failed","details":{"reason":{"oomKilled":{"memoryLimit":"512Mi"}}}}],"next_cursor":"c2"}`,
			[]string{`"server_failed"`, `"oomKilled"`, `"c2"`}, []string{"serviceId"}},
		{"render_list_logs", `{"hasMore":true,"nextStartTime":"2026-11-01T00:00:00Z","nextEndTime":"2026-11-01T01:00:00Z","logs":[{"id":"log-uuid-1","timestamp":"2026-11-01T00:30:00Z","message":"boom","labels":[{"name":"level","value":"error"}]}]}`,
			[]string{`"boom"`, `"error"`, `"hasMore"`, `"nextStartTime"`}, []string{"log-uuid-1"}},
		{"render_list_postgres_instances", `[{"id":"dpg-1","name":"db","status":"available","plan":"basic_1gb","region":"oregon","version":"16","databaseName":"app","databaseUser":"app_user","owner":{"id":"tea-1","email":"a@b.c"},"ipAllowList":[{"cidrBlock":"1.2.3.4/32"}],"readReplicas":[]}]`,
			[]string{`"dpg-1"`, `"available"`, `"basic_1gb"`, `"app"`}, []string{"owner", "ipAllowList", "app_user"}},
		{"render_get_postgres", `{"id":"dpg-1","status":"available","databaseUser":"app_user","owner":{"email":"owner@x"},"ipAllowList":[{"cidrBlock":"1.2.3.4/32"}]}`,
			[]string{`"app_user"`}, []string{"owner@x", "cidrBlock"}},
		{"render_list_key_value", `[{"id":"red-1","name":"cache","status":"available","plan":"starter","region":"oregon","version":"8","options":{"maxmemoryPolicy":"allkeys_lru"},"owner":{"email":"owner@x"},"ipAllowList":[{"cidrBlock":"0.0.0.0/0"}]}]`,
			[]string{`"red-1"`, `"allkeys_lru"`}, []string{"owner@x", "cidrBlock"}},
		{"render_get_key_value", `{"id":"red-1","options":{"persistenceMode":"snapshot"},"owner":{"email":"owner@x"}}`,
			[]string{`"snapshot"`}, []string{"owner@x"}},
		{"render_get_blueprint", `{"id":"exs-1","name":"stack","status":"in_sync","repo":"https://github.com/a/b","branch":"main","path":"render.yaml","autoSync":true,"ownerId":"tea-1","resources":[{"id":"srv-1","type":"web_service","name":"api"}]}`,
			[]string{`"exs-1"`, `"in_sync"`, `"srv-1"`}, []string{"ownerId"}},
	}
	r := &render{}
	for _, tc := range tests {
		t.Run(string(tc.tool), func(t *testing.T) {
			fields, ok := r.CompactSpec(tc.tool)
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
