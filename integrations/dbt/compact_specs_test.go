package dbt

import (
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

func TestFieldCompactionSpecs_PassthroughReadToolsCovered(t *testing.T) {
	for _, name := range []string{
		"dbt_list_jobs_runs",
	} {
		_, ok := fieldCompactionSpecs[mcp.ToolName(name)]
		assert.True(t, ok, "read tool %s should have a compaction spec", name)
	}
}

func TestFieldCompactionSpec_UnknownTool(t *testing.T) {
	_, ok := (&dbt{}).CompactSpec("dbt_nonexistent")
	assert.False(t, ok)
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"dbt_list_jobs_runs": `{"runs":[{"id":100,"job_definition_id":7,"status":20,"status_humanized":"Error","git_sha":"abc","git_branch":"master","created_at":"x","finished_at":"y","duration_humanized":"5m","trigger":{"cause":"Scheduled","id":9},"href":"https://cloud.getdbt.com/runs/100","account_id":1,"owner_thread_id":"zzz"}],"total_count":1}`,
	}
	for toolName, payload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			out := string(compacted)
			assert.NotEqual(t, "{}", out)
			assert.NotContains(t, out, "owner_thread_id")
			assert.NotContains(t, out, "account_id")
		})
	}
}

func TestFieldCompactionSpecs_RunsKeepFollowUpKeys(t *testing.T) {
	fields := fieldCompactionSpecs["dbt_list_jobs_runs"]
	compacted, err := mcp.CompactJSON([]byte(`{"runs":[{"id":100,"status":20,"git_sha":"abc","trigger":{"cause":"Scheduled"}}],"total_count":1}`), fields)
	require.NoError(t, err)
	for _, want := range []string{`"id":100`, `"git_sha":"abc"`, `"cause":"Scheduled"`, `"status":20`} {
		assert.Contains(t, string(compacted), want)
	}
}
