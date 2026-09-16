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

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		require.True(t, ok, "orphan compact spec %s", name)
	}
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	fields, ok := fieldCompactionSpecs["gitlab_list_merge_requests"]
	require.True(t, ok)
	payload := `[{"iid":1,"title":"Fix","state":"opened","web_url":"https://gitlab.com/mr/1","author":{"username":"alice"},"noise":"drop"}]`
	compacted, err := mcp.CompactJSON([]byte(payload), fields)
	require.NoError(t, err)
	assert.Contains(t, string(compacted), "Fix")
	assert.NotContains(t, string(compacted), "noise")
}
