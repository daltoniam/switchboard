package aws

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
	mutationPrefixes := []string{"put", "delete", "send", "start", "stop", "publish", "purge"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	a := &integration{}
	fields, ok := a.CompactSpec("aws_get_caller_identity")
	require.True(t, ok, "aws_get_caller_identity should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	a := &integration{}
	_, ok := a.CompactSpec("aws_s3_delete_object")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	a := &integration{}
	_, ok := a.CompactSpec("aws_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}
