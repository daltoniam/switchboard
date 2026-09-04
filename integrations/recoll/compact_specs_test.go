package recoll

import (
	"testing"

	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var specFile compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &specFile))
	assert.Equal(t, len(specFile.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_SearchShape(t *testing.T) {
	fields, ok := New().(*recoll).CompactSpec("recoll_search")
	assert.True(t, ok)
	assert.NotEmpty(t, fields)
}
