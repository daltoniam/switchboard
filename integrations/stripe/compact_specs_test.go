package stripe

import (
	"strings"
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

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	s := &stripe{}
	fields, ok := s.CompactSpec("stripe_list_customers")
	require.True(t, ok, "stripe_list_customers should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	s := &stripe{}
	_, ok := s.CompactSpec("stripe_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	s := &stripe{}
	_, ok := s.CompactSpec("stripe_create_customer")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

// List-shape specs should include "object" and "has_more" so the LLM can paginate.
func TestFieldCompactionSpecs_ListShapeIncludesPaginationFields(t *testing.T) {
	for toolName, raw := range rawFieldCompactionSpecs {
		name := string(toolName)
		if !strings.HasPrefix(name, "stripe_list_") && !strings.HasPrefix(name, "stripe_search_") {
			continue
		}
		joined := strings.Join(raw, ",")
		assert.Contains(t, joined, "object", "list/search spec %s missing 'object'", toolName)
		assert.Contains(t, joined, "has_more", "list/search spec %s missing 'has_more'", toolName)
	}
}

// All compaction-spec tool names must reference defined tools.
func TestFieldCompactionSpecs_AllToolsDefined(t *testing.T) {
	i := New()
	defined := map[mcp.ToolName]bool{}
	for _, tool := range i.Tools() {
		defined[tool.Name] = true
	}
	for toolName := range rawFieldCompactionSpecs {
		assert.True(t, defined[toolName], "compaction spec for undefined tool: %s", toolName)
	}
}
