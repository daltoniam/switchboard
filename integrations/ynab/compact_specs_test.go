package ynab

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
	y := &ynab{}
	fields, ok := y.CompactSpec("ynab_list_budgets")
	require.True(t, ok, "ynab_list_budgets should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	y := &ynab{}
	_, ok := y.CompactSpec("ynab_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	y := &ynab{}
	_, ok := y.CompactSpec("ynab_create_transaction")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}
