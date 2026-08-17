package postgres

import (
	"testing"

	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
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

func TestFieldCompactionSpecs_OnlyReadTools(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		assert.NotContains(t, toolName, "_create_",
			"mutation tool %q should not have a field compaction spec", toolName)
		assert.NotContains(t, toolName, "_update_",
			"mutation tool %q should not have a field compaction spec", toolName)
		assert.NotContains(t, toolName, "_delete_",
			"mutation tool %q should not have a field compaction spec", toolName)
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	p := &postgres{}
	fields, ok := p.CompactSpec("postgres_list_schemas")
	require.True(t, ok, "postgres_list_schemas should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	p := &postgres{}
	_, ok := p.CompactSpec("postgres_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}
