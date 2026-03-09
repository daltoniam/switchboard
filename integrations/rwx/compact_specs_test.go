package rwx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	assert.Equal(t, len(rawFieldCompactionSpecs), len(fieldCompactionSpecs))
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
	r := &rwx{}
	fields, ok := r.CompactSpec("rwx_get_recent_runs")
	require.True(t, ok, "rwx_get_recent_runs should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	r := &rwx{}
	_, ok := r.CompactSpec("rwx_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}
