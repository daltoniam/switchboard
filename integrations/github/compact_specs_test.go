package github

import (
	"testing"

	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	// fieldCompactionSpecs is loaded from compact.yaml at package init.
	// If any spec were invalid, lenient-mode loading would skip it with a warning.
	// This test verifies the map is populated.
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

// TestFieldCompactionSpecs_NoDuplicateTools verifies the YAML loader did not
// silently drop any tool entries. The YAML format prevents duplicate keys at
// the parser level; this confirms parse losslessness.
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

func TestFieldCompactionSpecs_OnlyListTools(t *testing.T) {
	// Compact specs should only be on list/read tools, not mutations.
	mutationPrefixes := []string{"create", "update", "delete", "merge", "lock", "unlock", "add", "remove", "rerun", "cancel", "request"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	g := &integration{}
	fields, ok := g.CompactSpec("github_list_issues")
	require.True(t, ok, "github_list_issues should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	g := &integration{}
	_, ok := g.CompactSpec("github_create_issue")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	g := &integration{}
	_, ok := g.CompactSpec("github_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForGetTools(t *testing.T) {
	g := &integration{}
	// Get tools are the full-detail fallback — no field compaction.
	_, ok := g.CompactSpec("github_get_issue")
	assert.False(t, ok, "get tools should return false (full detail)")
}
