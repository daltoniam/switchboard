package airflow

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	var spec compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &spec))
	assert.Len(t, fieldCompactionSpecs, len(spec.Tools))
	for tool := range fieldCompactionSpecs {
		assert.NotNil(t, dispatch[tool])
	}
	for _, tool := range New().Tools() {
		_, exists := fieldCompactionSpecs[tool.Name]
		assert.Equal(t, tool.Name != "airflow_trigger_dag", exists, "%s", tool.Name)
	}
}

func TestFieldCompactionSpecs_Shape(t *testing.T) {
	for _, tt := range []struct {
		tool  mcp.ToolName
		input string
		want  string
	}{
		{"airflow_list_dags", `{"dags":[{"dag_id":"nightly","is_paused":false,"secrets":"drop"}],"total_entries":1}`, "nightly"},
		{"airflow_get_dag", `{"dag_id":"nightly","is_active":true,"secrets":"drop"}`, "nightly"},
		{"airflow_list_dag_runs", `{"dag_runs":[{"dag_id":"nightly","dag_run_id":"manual__1","state":"failed","conf":{"key":"secret"}}],"total_entries":1}`, "failed"},
		{"airflow_get_dag_run", `{"dag_id":"nightly","dag_run_id":"manual__1","state":"success","conf":{"key":"secret"}}`, "success"},
		{"airflow_list_task_instances", `{"task_instances":[{"task_id":"extract","state":"failed","operator":"secret"}],"total_entries":1}`, "extract"},
	} {
		t.Run(string(tt.tool), func(t *testing.T) {
			fields, found := New().(*airflow).CompactSpec(tt.tool)
			require.True(t, found)
			out, err := mcp.CompactJSON([]byte(tt.input), fields)
			require.NoError(t, err)
			assert.Contains(t, string(out), tt.want)
			assert.NotContains(t, string(out), "secret")
		})
	}
}
