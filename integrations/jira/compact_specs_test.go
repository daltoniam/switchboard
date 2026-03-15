package jira

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
	mutationPrefixes := []string{"create", "update", "delete", "assign", "transition", "move", "add"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForReadTool(t *testing.T) {
	j := &jira{}
	fields, ok := j.CompactSpec("jira_search_issues")
	require.True(t, ok, "jira_search_issues should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	j := &jira{}
	_, ok := j.CompactSpec("jira_create_issue")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	j := &jira{}
	_, ok := j.CompactSpec("jira_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ShapeParity_SearchIssues(t *testing.T) {
	// Jira /rest/api/3/search returns {issues: [...], total: N}
	payload := `{"issues":[{"key":"PROJ-1","fields":{"summary":"Fix bug","status":{"name":"Open"},"assignee":{"displayName":"Alice"},"priority":{"name":"High"},"issuetype":{"name":"Bug"},"created":"2024-01-01","updated":"2024-01-02"}}],"total":1}`
	fields, ok := fieldCompactionSpecs["jira_search_issues"]
	require.True(t, ok)
	compacted, err := mcp.CompactJSON([]byte(payload), fields)
	require.NoError(t, err)
	assert.NotEqual(t, "{}", string(compacted))
	assert.Contains(t, string(compacted), "PROJ-1")
}
