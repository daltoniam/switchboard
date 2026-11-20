package growthbook

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// fixtureCases pairs each read tool with a response captured from a real
// GrowthBook 5.2 server seeded the same way as web/live_growthbook_test.go.
var fixtureCases = []struct {
	tool    mcp.ToolName
	fixture string
	args    map[string]any
}{
	{"growthbook_list_features", "features.json", nil},
	{"growthbook_get_feature", "feature.json", map[string]any{"feature_id": "live-beta-banner"}},
	{"growthbook_get_feature_staleness", "stale.json", map[string]any{"feature_ids": "live-checkout-express"}},
	{"growthbook_list_feature_revisions", "revisions.json", map[string]any{"feature_id": "live-checkout-express", "status": "published"}},
	{"growthbook_list_experiments", "experiments.json", nil},
	{"growthbook_get_experiment", "experiment.json", map[string]any{"experiment_id": "exp_1"}},
	{"growthbook_get_experiment_results", "results.json", map[string]any{"experiment_id": "exp_1"}},
	{"growthbook_get_snapshot", "snapshot.json", map[string]any{"snapshot_id": "snp_1"}},
	{"growthbook_list_fact_metrics", "factmetrics.json", nil},
	{"growthbook_get_fact_metric", "factmetric.json", map[string]any{"metric_id": "fact__1"}},
	{"growthbook_list_metrics", "metrics.json", nil},
	{"growthbook_list_fact_tables", "facttables.json", nil},
	{"growthbook_list_projects", "projects.json", nil},
	{"growthbook_list_environments", "envs.json", nil},
	{"growthbook_list_sdk_connections", "sdk.json", nil},
	{"growthbook_list_saved_groups", "savedgroups.json", nil},
	{"growthbook_get_saved_group", "savedgroup.json", map[string]any{"saved_group_id": "grp_1"}},
	{"growthbook_list_attributes", "attributes.json", nil},
	{"growthbook_list_data_sources", "datasources.json", nil},
	{"growthbook_list_segments", "segments.json", nil},
	{"growthbook_list_audit_logs", "audits.json", map[string]any{"entity_type": "feature"}},
	{"growthbook_list_code_refs", "coderefs.json", nil},
	{"growthbook_get_feature_code_refs", "coderef.json", map[string]any{"feature_id": "live-checkout-express"}},
	{"growthbook_get_settings", "settings.json", nil},
	{"growthbook_list_members", "members.json", nil},
}

func handlerOutput(t *testing.T, tool mcp.ToolName, fixture string, args map[string]any) any {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	result, err := adapter.Execute(context.Background(), tool, args)
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	var out any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	return out
}

func fieldPresent(raw any, field mcp.CompactField) bool {
	source, _, _ := strings.Cut(field.Spec(), ":")
	if !strings.ContainsAny(source, ".[*") {
		object, ok := raw.(map[string]any)
		if !ok {
			return false
		}
		_, found := object[source]
		return found
	}
	return !emptyValue(mcp.CompactAny(raw, []mcp.CompactField{field}))
}

func emptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case map[string]any:
		for _, item := range typed {
			if !emptyValue(item) {
				return false
			}
		}
		return true
	case []any:
		for _, item := range typed {
			if !emptyValue(item) {
				return false
			}
		}
		return true
	}
	return false
}

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var file compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &file))
	assert.Equal(t, len(file.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", name)
	}
}

func TestFieldCompactionSpecs_ReadToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		_, ok := fieldCompactionSpecs[tool.Name]
		if tool.Access == mcp.AccessRead {
			assert.True(t, ok, "read tool %s needs a compaction spec", tool.Name)
			continue
		}
		assert.False(t, ok, "write tool %s must return its full confirmation", tool.Name)
	}
}

func TestCompactSpecAndMaxBytes(t *testing.T) {
	adapter := New().(*growthbook)
	fields, ok := adapter.CompactSpec("growthbook_list_features")
	require.True(t, ok)
	assert.NotEmpty(t, fields)
	_, ok = adapter.CompactSpec("growthbook_toggle_feature")
	assert.False(t, ok)
	limit, ok := adapter.MaxBytes("growthbook_get_experiment_results")
	require.True(t, ok)
	assert.Positive(t, limit)
}

func TestFixtureCases_CoverEveryReadTool(t *testing.T) {
	covered := map[mcp.ToolName]bool{}
	for _, c := range fixtureCases {
		covered[c.tool] = true
	}
	for _, tool := range New().Tools() {
		if tool.Access == mcp.AccessRead {
			assert.True(t, covered[tool.Name], "read tool %s has no captured fixture", tool.Name)
		}
	}
}

// Every compaction field must match the handler output for a real response,
// so a renamed upstream field cannot silently strip data from every call.
func TestFieldCompactionSpecs_MatchRealResponses(t *testing.T) {
	for _, c := range fixtureCases {
		t.Run(string(c.tool), func(t *testing.T) {
			raw := handlerOutput(t, c.tool, c.fixture, c.args)
			fields, ok := fieldCompactionSpecs[c.tool]
			require.True(t, ok)
			for _, field := range fields {
				assert.True(t, fieldPresent(raw, field), "field %q matches nothing in %s", field.Spec(), c.fixture)
			}
			compacted, err := json.Marshal(mcp.CompactAny(raw, fields))
			require.NoError(t, err)
			assert.NotContains(t, []string{"{}", "[]", "null"}, string(compacted))
		})
	}
}

func TestFieldCompaction_KeepsDecisionData(t *testing.T) {
	tests := []struct {
		tool    mcp.ToolName
		fixture string
		args    map[string]any
		want    []string
		absent  []string
	}{
		{"growthbook_list_features", "features.json", nil, []string{"live-checkout-express", `"enabled":true`, "Beta testers only", `"coverage":0.1`}, []string{"definition", "apiKey"}},
		{"growthbook_get_feature", "feature.json", map[string]any{"feature_id": "live-beta-banner"}, []string{"grp_", "rollout", `"production"`}, []string{"definition"}},
		{"growthbook_get_experiment_results", "results.json", map[string]any{"experiment_id": "exp_1"}, []string{"chanceToBeatControl", "Express", "srm", "totalUsers"}, []string{"queryIds"}},
		{"growthbook_list_sdk_connections", "sdk.json", nil, []string{"Live Web", "sdk-"}, []string{"SECRET_", "encryptionKey", "proxySigningKey"}},
		{"growthbook_get_settings", "settings.json", nil, []string{"bayesian"}, []string{"secureAttributeSalt"}},
		{"growthbook_list_audit_logs", "audits.json", map[string]any{"entity_type": "feature"}, []string{"feature.toggle", "live launch"}, []string{"details", "apiKey"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.tool), func(t *testing.T) {
			raw := handlerOutput(t, tt.tool, tt.fixture, tt.args)
			compacted, err := json.Marshal(mcp.CompactAny(raw, fieldCompactionSpecs[tt.tool]))
			require.NoError(t, err)
			for _, want := range tt.want {
				assert.Contains(t, string(compacted), want)
			}
			for _, absent := range tt.absent {
				assert.NotContains(t, string(compacted), absent)
			}
		})
	}
}

func TestHandlers_StripSecretsBeforeScripts(t *testing.T) {
	for _, c := range []struct {
		tool    mcp.ToolName
		fixture string
		absent  []string
	}{
		{"growthbook_list_sdk_connections", "sdk.json", []string{"encryptionKey", "proxySigningKey", "SECRET_"}},
		{"growthbook_get_settings", "settings.json", []string{"secureAttributeSalt"}},
		{"growthbook_list_features", "features.json", []string{"definition"}},
		{"growthbook_get_feature", "feature.json", []string{"definition"}},
	} {
		raw := handlerOutput(t, c.tool, c.fixture, map[string]any{"feature_id": "live-beta-banner"})
		data, err := json.Marshal(raw)
		require.NoError(t, err)
		for _, absent := range c.absent {
			assert.NotContains(t, string(data), absent, "%s leaks %s", c.tool, absent)
		}
	}
}
