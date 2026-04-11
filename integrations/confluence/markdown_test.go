package confluence

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_Page(t *testing.T) {
	c := &confluence{}
	data := `{"id":"100","title":"Page One","spaceId":"DEV","authorId":"user1","createdAt":"2024-01-01","version":{"number":3},"body":{"storage":{"value":"<p>Hello world</p>"}}}`

	md, ok := c.RenderMarkdown("confluence_get_page", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "<!-- confluence:page_id=100 space=DEV version=3 -->")
	assert.Contains(t, string(md), "# Page One")
	assert.Contains(t, string(md), "Hello world")
	assert.Contains(t, string(md), "Author: user1")
}

func TestRenderMarkdown_BlogPost(t *testing.T) {
	c := &confluence{}
	data := `{"id":"200","title":"Blog Post","spaceId":"DEV","authorId":"user1","createdAt":"2024-01-01","version":{"number":1},"body":{"storage":{"value":"<p>Blog content</p>"}}}`

	md, ok := c.RenderMarkdown("confluence_get_blog_post", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "# Blog Post")
	assert.Contains(t, string(md), "Blog content")
}

func TestRenderMarkdown_Comments(t *testing.T) {
	c := &confluence{}
	data := `{"results":[{"authorId":"user1","createdAt":"2024-01-14","version":{"number":2},"body":{"storage":{"value":"<p>Updated the goals.</p>"}}},{"authorId":"user2","createdAt":"2024-01-14","version":{"number":2},"body":{"storage":{"value":"<p>Looks good!</p>"}}}]}`

	md, ok := c.RenderMarkdown("confluence_list_comments", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "## Comments (2)")
	assert.Contains(t, string(md), "**user1**")
	assert.Contains(t, string(md), "Updated the goals.")
	assert.Contains(t, string(md), "**user2**")
	assert.Contains(t, string(md), "Looks good!")
}

func TestRenderMarkdown_EmptyComments(t *testing.T) {
	c := &confluence{}
	data := `{"results":[]}`

	md, ok := c.RenderMarkdown("confluence_list_comments", []byte(data))
	require.True(t, ok)
	assert.Equal(t, markdown.Markdown("No comments.\n"), md)
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	c := &confluence{}
	_, ok := c.RenderMarkdown("confluence_search", []byte(`{}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	c := &confluence{}
	_, ok := c.RenderMarkdown("confluence_get_page", []byte(`not json`))
	assert.False(t, ok)
}

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	adapter := New()
	md, ok := adapter.(mcp.MarkdownIntegration)
	require.True(t, ok, "adapter should implement MarkdownIntegration")

	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range adapter.Tools() {
		toolNames[tool.Name] = true
	}

	// Test every tool — verify RenderMarkdown returns ok=true only for known tools
	for name := range toolNames {
		// We just check it doesn't panic; the (_, ok) result depends on the tool
		md.RenderMarkdown(name, []byte("{}"))
	}

	// Verify the tools RenderMarkdown claims to handle actually exist
	markdownTools := []mcp.ToolName{
		"confluence_get_page",
		"confluence_get_blog_post",
		"confluence_list_comments",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
