package datadog

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
	mutationPrefixes := []string{"create", "update", "delete", "mute", "unmute"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	d := &dd{}
	fields, ok := d.CompactSpec("datadog_search_logs")
	require.True(t, ok, "datadog_search_logs should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	d := &dd{}
	_, ok := d.CompactSpec("datadog_create_monitor")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	d := &dd{}
	_, ok := d.CompactSpec("datadog_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}
