package gdrive

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_File(t *testing.T) {
	g := &gdrive{}
	data := `{
		"id": "f1",
		"name": "Quarterly Report.docx",
		"mimeType": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"description": "Q1 numbers + summary.",
		"size": "12345",
		"modifiedTime": "2024-03-15T14:00:00Z",
		"createdTime": "2024-03-10T09:00:00Z",
		"webViewLink": "https://drive.google.com/file/d/f1/view",
		"owners": [
			{"emailAddress": "alice@example.com", "displayName": "Alice"}
		],
		"shared": true,
		"starred": true
	}`

	md, ok := g.RenderMarkdown("gdrive_get_file", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "<!-- gdrive:file_id=f1")
	assert.Contains(t, s, "# Quarterly Report.docx")
	assert.Contains(t, s, "Type: application/vnd.openxmlformats")
	assert.Contains(t, s, "Size: 12345 bytes")
	assert.Contains(t, s, "Modified: 2024-03-15T14:00:00Z")
	assert.Contains(t, s, "Owners: Alice <alice@example.com>")
	assert.Contains(t, s, "Link: https://drive.google.com/file/d/f1/view")
	assert.Contains(t, s, "Flags:")
	assert.Contains(t, s, "starred")
	assert.Contains(t, s, "shared")
	assert.Contains(t, s, "## Description")
	assert.Contains(t, s, "Q1 numbers + summary.")
}

func TestRenderMarkdown_File_NoName(t *testing.T) {
	g := &gdrive{}
	md, ok := g.RenderMarkdown("gdrive_get_file", []byte(`{"id":"f2"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "(no name)")
}

func TestRenderMarkdown_Download_Text(t *testing.T) {
	g := &gdrive{}
	data := `{"content_type":"text/plain","bytes":11,"truncated":false,"content":"hello world"}`
	md, ok := g.RenderMarkdown("gdrive_download_file", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "<!-- gdrive:content_type=text/plain")
	assert.Contains(t, s, "```")
	assert.Contains(t, s, "hello world")
}

func TestRenderMarkdown_Download_Markdown(t *testing.T) {
	g := &gdrive{}
	data := `{"content_type":"text/markdown","bytes":13,"truncated":false,"content":"# Heading\n\nBody"}`
	md, ok := g.RenderMarkdown("gdrive_export_file", []byte(data))
	require.True(t, ok)
	s := string(md)
	// Markdown content should pass through as markdown body, not in a code fence
	assert.Contains(t, s, "# Heading")
	assert.Contains(t, s, "Body")
}

func TestRenderMarkdown_Download_Binary(t *testing.T) {
	g := &gdrive{}
	data := `{"content_type":"image/png","bytes":42,"truncated":false,"content_base64":"iVBORw0KGgo="}`
	md, ok := g.RenderMarkdown("gdrive_download_file", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Binary content")
	// Make sure we don't dump the base64 into the markdown body.
	assert.NotContains(t, s, "iVBORw0KGgo=")
}

func TestRenderMarkdown_Download_Truncated(t *testing.T) {
	g := &gdrive{}
	data := `{"content_type":"text/plain","bytes":10,"truncated":true,"content":"aaaaaaaaaa"}`
	md, ok := g.RenderMarkdown("gdrive_download_file", []byte(data))
	require.True(t, ok)
	assert.Contains(t, string(md), "truncated")
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gdrive{}
	_, ok := g.RenderMarkdown("gdrive_list_files", []byte(`{}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	g := &gdrive{}
	_, ok := g.RenderMarkdown("gdrive_get_file", []byte(`{bad`))
	assert.False(t, ok)
}

func TestRenderMarkdown_File_MissingID(t *testing.T) {
	// JSON parses but doesn't look like a file resource — skip rendering.
	g := &gdrive{}
	_, ok := g.RenderMarkdown("gdrive_get_file", []byte(`{"foo":"bar"}`))
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

	// Call RenderMarkdown for every tool — should not panic
	for name := range toolNames {
		md.RenderMarkdown(name, []byte("{}"))
	}

	// Verify the tools RenderMarkdown claims to handle actually exist
	markdownTools := []mcp.ToolName{
		"gdrive_get_file",
		"gdrive_download_file",
		"gdrive_export_file",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
