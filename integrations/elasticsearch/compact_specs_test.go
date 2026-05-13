package elasticsearch

import (
	"testing"

	"github.com/daltoniam/switchboard/compactyaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compactyaml.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_OnlyReadTools(t *testing.T) {
	mutationPrefixes := []string{"_create_", "_delete_", "_put_", "_update_", "_open_", "_close_", "_refresh_", "_forcemerge_", "_cancel_", "_reindex"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, prefix,
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	e := &esInt{}
	fields, ok := e.CompactSpec("elasticsearch_list_indices")
	require.True(t, ok, "elasticsearch_list_indices should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	e := &esInt{}
	_, ok := e.CompactSpec("elasticsearch_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}
