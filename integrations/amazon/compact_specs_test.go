package amazon

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

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	a := &amazon{}
	fields, ok := a.CompactSpec("amazon_search_products")
	require.True(t, ok, "amazon_search_products should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	a := &amazon{}
	_, ok := a.CompactSpec("amazon_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	a := &amazon{}
	_, ok := a.CompactSpec("amazon_add_to_cart")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestFieldCompactionSpec_NoMissingReadToolSpecs(t *testing.T) {
	readTools := map[string]bool{
		"amazon_search_products": true,
		"amazon_get_product":     true,
		"amazon_get_orders":      true,
		"amazon_get_cart":        true,
	}
	for toolName := range readTools {
		_, ok := fieldCompactionSpecs[toolName]
		assert.True(t, ok, "read tool %q should have a field compaction spec", toolName)
	}
}
