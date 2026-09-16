package gitlab

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
	var specFile compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &specFile))
	assert.Equal(t, len(specFile.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	tests := []struct {
		name     string
		toolName mcp.ToolName
		payload  string
		want     string
	}{
		{
			name:     "list_merge_requests",
			toolName: "gitlab_list_merge_requests",
			payload:  `{"merge_requests":[{"iid":1,"title":"Fix login","state":"opened","web_url":"https://gitlab.com/mr/1","author":{"username":"alice","name":"Alice"},"noise":"drop"}]}`,
			want:     "Fix login",
		},
		{
			name:     "get_job",
			toolName: "gitlab_get_job",
			payload:  `{"id":88,"name":"rspec","status":"failed","trace":"ERROR: failed","extra":"drop"}`,
			want:     "ERROR: failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[test.toolName]
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(test.payload), fields)
			require.NoError(t, err)
			assert.Contains(t, string(compacted), test.want)
			assert.NotContains(t, string(compacted), "noise")
			assert.NotContains(t, string(compacted), "extra")
		})
	}
}
