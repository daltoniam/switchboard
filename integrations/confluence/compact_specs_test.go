package confluence

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
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
	mutationPrefixes := []string{"create", "update", "delete"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForReadTool(t *testing.T) {
	c := &confluence{}
	fields, ok := c.CompactSpec("confluence_list_spaces")
	require.True(t, ok, "confluence_list_spaces should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	c := &confluence{}
	_, ok := c.CompactSpec("confluence_create_page")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	c := &confluence{}
	_, ok := c.CompactSpec("confluence_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	// Representative payloads matching the real Confluence API response shapes.
	handlerOutputs := map[string]string{
		// Spaces
		"confluence_list_spaces": `{"results":[{"id":"123","key":"DEV","name":"Development","type":"global","status":"current"}]}`,
		"confluence_get_space":   `{"id":"123","key":"DEV","name":"Development","type":"global","status":"current","description":{"plain":{"value":"Dev space"}},"homepageId":"456"}`,
		"confluence_search":      `{"results":[{"content":{"id":"789","type":"page","title":"Design Doc","status":"current","_links":{"webui":"/spaces/DEV/pages/789"}},"excerpt":"matching text","url":"/spaces/DEV/pages/789"}]}`,

		// Pages
		"confluence_list_pages":        `{"results":[{"id":"100","title":"Page One","status":"current","spaceId":"123","authorId":"user1","createdAt":"2024-01-01T00:00:00Z","version":{"number":1}}]}`,
		"confluence_get_page":          `{"id":"100","title":"Page One","status":"current","spaceId":"123","authorId":"user1","createdAt":"2024-01-01T00:00:00Z","version":{"number":1,"createdAt":"2024-01-01T00:00:00Z"},"body":{"storage":{"value":"<p>Content</p>"}},"parentId":"50","parentType":"page","_links":{"webui":"/spaces/DEV/pages/100"}}`,
		"confluence_get_page_children": `{"results":[{"id":"101","title":"Child Page","status":"current","spaceId":"123","authorId":"user1","createdAt":"2024-01-02T00:00:00Z","version":{"number":1}}]}`,

		// Blog Posts
		"confluence_list_blog_posts": `{"results":[{"id":"200","title":"Blog Post","status":"current","spaceId":"123","authorId":"user1","createdAt":"2024-01-01T00:00:00Z","version":{"number":1}}]}`,
		"confluence_get_blog_post":   `{"id":"200","title":"Blog Post","status":"current","spaceId":"123","authorId":"user1","createdAt":"2024-01-01T00:00:00Z","version":{"number":1,"createdAt":"2024-01-01T00:00:00Z"},"body":{"storage":{"value":"<p>Blog content</p>"}},"_links":{"webui":"/spaces/DEV/blog/200"}}`,

		// Comments
		"confluence_list_comments": `{"results":[{"id":"300","status":"current","authorId":"user1","createdAt":"2024-01-01T00:00:00Z","version":{"number":1},"body":{"storage":{"value":"<p>A comment</p>"}}}]}`,
	}

	for toolName, payload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok, "missing compaction spec for %s", toolName)
			compacted, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			assert.NotEqual(t, "{}", string(compacted), "compaction returned empty object — spec paths likely don't match response shape")
			assert.NotEqual(t, "[]", string(compacted), "compaction returned empty array — spec paths likely don't match response shape")
		})
	}
}
