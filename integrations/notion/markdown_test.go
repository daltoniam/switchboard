package notion

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRichTextToMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input []any
		want  string
	}{
		{name: "nil input", input: nil, want: ""},
		{name: "empty array", input: []any{}, want: ""},
		{name: "plain text", input: []any{[]any{"Hello world"}}, want: "Hello world"},
		{name: "bold", input: []any{[]any{"bold", []any{[]any{"b"}}}}, want: "**bold**"},
		{name: "italic", input: []any{[]any{"italic", []any{[]any{"i"}}}}, want: "*italic*"},
		{name: "code", input: []any{[]any{"code", []any{[]any{"c"}}}}, want: "`code`"},
		{name: "strikethrough", input: []any{[]any{"strike", []any{[]any{"s"}}}}, want: "~~strike~~"},
		{name: "link", input: []any{[]any{"click here", []any{[]any{"a", "https://example.com"}}}}, want: "[click here](https://example.com)"},
		{
			name:  "bold and italic combined",
			input: []any{[]any{"both", []any{[]any{"b"}, []any{"i"}}}},
			want:  "***both***",
		},
		{
			name:  "multiple segments",
			input: []any{[]any{"Hello "}, []any{"world", []any{[]any{"b"}}}},
			want:  "Hello **world**",
		},
		{
			name:  "three segments mixed",
			input: []any{[]any{"Start "}, []any{"bold", []any{[]any{"b"}}}, []any{" end"}},
			want:  "Start **bold** end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := richTextToMarkdown(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBlocksToMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		blocks []renderedBlock
		want   string
	}{
		{name: "empty blocks", blocks: nil, want: ""},
		{
			name:   "header with id",
			blocks: []renderedBlock{{ID: "h1", Type: "header", Text: "Title"}},
			want:   "<!-- block:h1 -->\n# Title\n",
		},
		{
			name:   "sub_header",
			blocks: []renderedBlock{{ID: "h2", Type: "sub_header", Text: "Subtitle"}},
			want:   "<!-- block:h2 -->\n## Subtitle\n",
		},
		{
			name:   "sub_sub_header",
			blocks: []renderedBlock{{ID: "h3", Type: "sub_sub_header", Text: "Sub-subtitle"}},
			want:   "<!-- block:h3 -->\n### Sub-subtitle\n",
		},
		{
			name:   "text paragraph",
			blocks: []renderedBlock{{ID: "t1", Type: "text", Text: "A paragraph."}},
			want:   "<!-- block:t1 -->\nA paragraph.\n",
		},
		{
			name:   "text without content renders empty line",
			blocks: []renderedBlock{{Type: "text"}},
			want:   "\n",
		},
		{
			name: "bulleted list",
			blocks: []renderedBlock{
				{ID: "b1", Type: "bulleted_list", Text: "Item one"},
				{ID: "b2", Type: "bulleted_list", Text: "Item two"},
			},
			want: "<!-- block:b1 -->\n- Item one\n<!-- block:b2 -->\n- Item two\n",
		},
		{
			name: "numbered list",
			blocks: []renderedBlock{
				{ID: "n1", Type: "numbered_list", Text: "First"},
				{ID: "n2", Type: "numbered_list", Text: "Second"},
			},
			want: "<!-- block:n1 -->\n1. First\n<!-- block:n2 -->\n2. Second\n",
		},
		{
			name:   "to_do unchecked",
			blocks: []renderedBlock{{ID: "td1", Type: "to_do", Text: "Task"}},
			want:   "<!-- block:td1 -->\n- [ ] Task\n",
		},
		{
			name:   "to_do checked",
			blocks: []renderedBlock{{ID: "td2", Type: "to_do", Text: "Done task", IsChecked: true}},
			want:   "<!-- block:td2 -->\n- [x] Done task\n",
		},
		{
			name:   "quote",
			blocks: []renderedBlock{{ID: "q1", Type: "quote", Text: "A wise saying"}},
			want:   "<!-- block:q1 -->\n> A wise saying\n",
		},
		{
			name:   "callout",
			blocks: []renderedBlock{{ID: "co1", Type: "callout", Text: "Important note"}},
			want:   "<!-- block:co1 -->\n> Important note\n",
		},
		{
			name:   "code block without language",
			blocks: []renderedBlock{{ID: "c1", Type: "code", Text: "fmt.Println(\"hello\")"}},
			want:   "<!-- block:c1 -->\n```\nfmt.Println(\"hello\")\n```\n",
		},
		{
			name:   "code block with language",
			blocks: []renderedBlock{{ID: "c2", Type: "code", Text: "def hello():\n    pass", CodeLanguage: "python"}},
			want:   "<!-- block:c2 -->\n```python\ndef hello():\n    pass\n```\n",
		},
		{
			name:   "divider has no id annotation",
			blocks: []renderedBlock{{ID: "d1", Type: "divider"}},
			want:   "---\n",
		},
		{
			name:   "toggle",
			blocks: []renderedBlock{{ID: "tg1", Type: "toggle", Text: "Click to expand"}},
			want:   "<!-- block:tg1 -->\n**Click to expand** (toggle)\n",
		},
		{
			name:   "block without id omits annotation",
			blocks: []renderedBlock{{Type: "header", Text: "No ID"}},
			want:   "# No ID\n",
		},
		{
			name: "mixed content with ids",
			blocks: []renderedBlock{
				{ID: "h1", Type: "header", Text: "My Doc"},
				{ID: "t1", Type: "text", Text: "First paragraph."},
				{ID: "b1", Type: "bulleted_list", Text: "Point A"},
				{ID: "b2", Type: "bulleted_list", Text: "Point B"},
				{ID: "d1", Type: "divider"},
				{ID: "t2", Type: "text", Text: "End."},
			},
			want: "<!-- block:h1 -->\n# My Doc\n\n<!-- block:t1 -->\nFirst paragraph.\n\n<!-- block:b1 -->\n- Point A\n<!-- block:b2 -->\n- Point B\n\n---\n\n<!-- block:t2 -->\nEnd.\n",
		},
		{
			name:   "unknown block type with id",
			blocks: []renderedBlock{{ID: "u1", Type: "some_future_block", Text: "Content"}},
			want:   "<!-- block:u1 -->\nContent\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blocksToMarkdown(tt.blocks)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCommentsToMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		threads []renderedThread
		want    markdown.Markdown
	}{
		{name: "no comments", threads: nil, want: "No comments.\n"},
		{
			name: "single thread open",
			threads: []renderedThread{
				{
					Resolved: false,
					Comments: []renderedComment{
						{Author: "user-1", CreatedAt: 1700000000000, Text: "Great work!"},
					},
				},
			},
			want: "## Comments (1 thread)\n\n### Thread 1 (open)\n> **user-1** (2023-11-14 22:13 UTC):\n> Great work!\n\n",
		},
		{
			name: "resolved thread",
			threads: []renderedThread{
				{
					Resolved: true,
					Comments: []renderedComment{
						{Author: "user-1", CreatedAt: 1700000000000, Text: "Fix this"},
						{Author: "user-2", CreatedAt: 1700000060000, Text: "Fixed"},
					},
				},
			},
			want: "## Comments (1 thread)\n\n### Thread 1 (resolved)\n> **user-1** (2023-11-14 22:13 UTC):\n> Fix this\n\n> **user-2** (2023-11-14 22:14 UTC):\n> Fixed\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commentsToMarkdown(tt.threads)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPageToMarkdown(t *testing.T) {
	page := renderedPage{
		ID:             "page-abc",
		Title:          "My Page",
		LastEditedTime: 1700000000000,
		Blocks: []renderedBlock{
			{ID: "blk-1", Type: "text", Text: "Hello world."},
		},
	}

	got := pageToMarkdown(page)
	assert.Contains(t, got, "<!-- notion:page_id=page-abc -->")
	assert.Contains(t, got, "# My Page")
	assert.Contains(t, got, "<!-- block:blk-1 -->")
	assert.Contains(t, got, "Hello world.")
}

func TestRenderMarkdown_EndToEnd(t *testing.T) {
	n := &notion{}

	t.Run("page content", func(t *testing.T) {
		data := `{"page":{"id":"p1","type":"page","properties":{"title":[["Test"]]},"last_edited_time":1700000000000},"blocks":[{"id":"b1","type":"text","properties":{"title":[["Hello"]]}}]}`
		md, ok := n.RenderMarkdown("notion_get_page_content", []byte(data))
		assert.True(t, ok)
		assert.Contains(t, md, "<!-- notion:page_id=p1 -->")
		assert.Contains(t, md, "# Test")
		assert.Contains(t, md, "Hello")
	})

	t.Run("comments", func(t *testing.T) {
		data := `{"results":[{"discussion":{"resolved":false},"comments":[{"created_by_id":"alice","created_time":1700000000000,"text":[["Nice work!"]]}]}]}`
		md, ok := n.RenderMarkdown("notion_retrieve_comments", []byte(data))
		assert.True(t, ok)
		assert.Contains(t, md, "## Comments")
		assert.Contains(t, md, "alice")
		assert.Contains(t, md, "Nice work!")
	})

	t.Run("unknown tool returns false", func(t *testing.T) {
		_, ok := n.RenderMarkdown("notion_search", []byte(`{}`))
		assert.False(t, ok)
	})
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
		"notion_get_page_content",
		"notion_retrieve_comments",
	}
	for _, name := range markdownTools {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}
