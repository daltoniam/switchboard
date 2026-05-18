package gdocs

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_BasicDocument(t *testing.T) {
	g := &gdocs{}
	data := `{
		"documentId": "doc-1",
		"title": "Weekly Notes",
		"revisionId": "rev-7",
		"body": {
			"content": [
				{
					"startIndex": 1, "endIndex": 12,
					"paragraph": {
						"paragraphStyle": {"namedStyleType": "HEADING_1"},
						"elements": [{"textRun": {"content": "Section A\n"}}]
					}
				},
				{
					"startIndex": 12, "endIndex": 30,
					"paragraph": {
						"paragraphStyle": {"namedStyleType": "NORMAL_TEXT"},
						"elements": [{"textRun": {"content": "Hello, world!\n"}}]
					}
				}
			]
		}
	}`
	md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "<!-- gdocs:document_id=doc-1")
	assert.Contains(t, s, "# Weekly Notes")
	assert.Contains(t, s, "# Section A")
	assert.Contains(t, s, "Hello, world!")
}

func TestRenderMarkdown_AllHeadingLevels(t *testing.T) {
	cases := []struct {
		style string
		want  string
	}{
		{"TITLE", "# Title text"},
		{"SUBTITLE", "## Title text"},
		{"HEADING_1", "# Title text"},
		{"HEADING_2", "## Title text"},
		{"HEADING_3", "### Title text"},
		{"HEADING_4", "#### Title text"},
		{"HEADING_5", "##### Title text"},
		{"HEADING_6", "###### Title text"},
	}
	for _, tc := range cases {
		t.Run(tc.style, func(t *testing.T) {
			data := `{
				"documentId": "x",
				"body": {
					"content": [{
						"paragraph": {
							"paragraphStyle": {"namedStyleType": "` + tc.style + `"},
							"elements": [{"textRun": {"content": "Title text\n"}}]
						}
					}]
				}
			}`
			g := &gdocs{}
			md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
			require.True(t, ok)
			assert.Contains(t, string(md), tc.want)
		})
	}
}

func TestRenderMarkdown_InlineStyles(t *testing.T) {
	cases := []struct {
		name      string
		textStyle string
		content   string
		wantSub   string
	}{
		{"bold", `"bold":true`, "bold text", "**bold text**"},
		{"italic", `"italic":true`, "italic text", "*italic text*"},
		{"bold+italic", `"bold":true,"italic":true`, "both", "***both***"},
		{"strikethrough", `"strikethrough":true`, "gone", "~~gone~~"},
		{"link", `"link":{"url":"https://example.com"}`, "click me", "[click me](https://example.com)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := `{
				"documentId": "x",
				"body": {
					"content": [{
						"paragraph": {
							"paragraphStyle": {"namedStyleType": "NORMAL_TEXT"},
							"elements": [{"textRun": {"content": "` + tc.content + `", "textStyle": {` + tc.textStyle + `}}}]
						}
					}]
				}
			}`
			g := &gdocs{}
			md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
			require.True(t, ok)
			assert.Contains(t, string(md), tc.wantSub)
		})
	}
}

func TestRenderMarkdown_BulletList(t *testing.T) {
	g := &gdocs{}
	data := `{
		"documentId": "x",
		"lists": {
			"list-1": {"listProperties": {"nestingLevels": [{"glyphType": "DISC"}]}}
		},
		"body": {
			"content": [
				{"paragraph": {"bullet": {"listId": "list-1", "nestingLevel": 0}, "elements": [{"textRun": {"content": "first\n"}}]}},
				{"paragraph": {"bullet": {"listId": "list-1", "nestingLevel": 1}, "elements": [{"textRun": {"content": "nested\n"}}]}}
			]
		}
	}`
	md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "- first")
	assert.Contains(t, s, "  - nested")
}

func TestRenderMarkdown_OrderedList(t *testing.T) {
	g := &gdocs{}
	data := `{
		"documentId": "x",
		"lists": {
			"list-1": {"listProperties": {"nestingLevels": [{"glyphType": "DECIMAL"}]}}
		},
		"body": {
			"content": [
				{"paragraph": {"bullet": {"listId": "list-1", "nestingLevel": 0}, "elements": [{"textRun": {"content": "one\n"}}]}},
				{"paragraph": {"bullet": {"listId": "list-1", "nestingLevel": 0}, "elements": [{"textRun": {"content": "two\n"}}]}}
			]
		}
	}`
	md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "1. one")
	assert.Contains(t, s, "1. two")
}

func TestRenderMarkdown_SimpleTable(t *testing.T) {
	g := &gdocs{}
	data := `{
		"documentId": "x",
		"body": {
			"content": [{
				"table": {
					"rows": 2,
					"columns": 2,
					"tableRows": [
						{"tableCells": [
							{"content": [{"paragraph": {"elements": [{"textRun": {"content": "Name"}}]}}]},
							{"content": [{"paragraph": {"elements": [{"textRun": {"content": "Age"}}]}}]}
						]},
						{"tableCells": [
							{"content": [{"paragraph": {"elements": [{"textRun": {"content": "Alice"}}]}}]},
							{"content": [{"paragraph": {"elements": [{"textRun": {"content": "30"}}]}}]}
						]}
					]
				}
			}]
		}
	}`
	md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "| Name | Age |")
	assert.Contains(t, s, "| --- | --- |")
	assert.Contains(t, s, "| Alice | 30 |")
}

func TestRenderMarkdown_SectionBreak(t *testing.T) {
	g := &gdocs{}
	data := `{
		"documentId": "x",
		"body": {
			"content": [
				{"paragraph": {"paragraphStyle": {"namedStyleType": "NORMAL_TEXT"}, "elements": [{"textRun": {"content": "before\n"}}]}},
				{"sectionBreak": {}},
				{"paragraph": {"paragraphStyle": {"namedStyleType": "NORMAL_TEXT"}, "elements": [{"textRun": {"content": "after\n"}}]}}
			]
		}
	}`
	md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "before")
	assert.Contains(t, s, "---")
	assert.Contains(t, s, "after")
}

func TestRenderMarkdown_UntitledFallback(t *testing.T) {
	g := &gdocs{}
	md, ok := g.RenderMarkdown("gdocs_get_document", []byte(`{"documentId":"x"}`))
	require.True(t, ok)
	assert.Contains(t, string(md), "(untitled document)")
}

func TestRenderMarkdown_MissingDocumentID(t *testing.T) {
	g := &gdocs{}
	// JSON parses but doesn't look like a document — skip rendering.
	_, ok := g.RenderMarkdown("gdocs_get_document", []byte(`{"foo":"bar"}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_InvalidJSON(t *testing.T) {
	g := &gdocs{}
	_, ok := g.RenderMarkdown("gdocs_get_document", []byte(`{bad`))
	assert.False(t, ok)
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gdocs{}
	_, ok := g.RenderMarkdown("gdocs_create_document", []byte(`{"documentId":"x"}`))
	assert.False(t, ok)
}

func TestRenderMarkdown_FootnoteReference(t *testing.T) {
	g := &gdocs{}
	data := `{
		"documentId": "x",
		"body": {
			"content": [{
				"paragraph": {
					"paragraphStyle": {"namedStyleType": "NORMAL_TEXT"},
					"elements": [
						{"textRun": {"content": "See note"}},
						{"footnoteReference": {"footnoteId": "fn1", "footnoteNumber": "1"}},
						{"textRun": {"content": " for details.\n"}}
					]
				}
			}]
		}
	}`
	md, ok := g.RenderMarkdown("gdocs_get_document", []byte(data))
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "See note")
	assert.Contains(t, s, "[^1]")
	assert.Contains(t, s, "for details.")
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
		"gdocs_get_document",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
