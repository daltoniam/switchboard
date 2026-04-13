package ollama

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
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
	mutationPrefixes := []string{"create", "update", "delete", "pull", "push", "copy"}
	for toolName := range fieldCompactionSpecs {
		name := string(toolName)
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, name, "_"+prefix+"_",
				"mutation tool %q should not have compaction specs", toolName)
		}
	}
}

func TestFieldCompactionSpecs_ExpectedTools(t *testing.T) {
	expected := []mcp.ToolName{"ollama_list_models", "ollama_list_running"}
	for _, name := range expected {
		_, ok := fieldCompactionSpecs[name]
		assert.True(t, ok, "expected compaction spec for %s", name)
	}
}
