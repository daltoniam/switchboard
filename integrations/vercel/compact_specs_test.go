package vercel

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

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	v := &vercel{}
	fields, ok := v.CompactSpec("vercel_list_projects")
	require.True(t, ok, "vercel_list_projects should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	v := &vercel{}
	_, ok := v.CompactSpec("vercel_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	v := &vercel{}
	_, ok := v.CompactSpec("vercel_create_project")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestMaxBytes_ReturnsCapForLogTool(t *testing.T) {
	v := &vercel{}
	n, ok := v.MaxBytes("vercel_list_deployment_events")
	require.True(t, ok)
	assert.Equal(t, 131072, n)
}
