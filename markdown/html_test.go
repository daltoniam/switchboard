package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Markdown
	}{
		{name: "empty", input: "", want: ""},
		{name: "plain paragraph", input: "<p>Hello world</p>", want: "Hello world\n\n"},
		{name: "two paragraphs", input: "<p>First</p><p>Second</p>", want: "First\n\nSecond\n\n"},
		{name: "h1", input: "<h1>Title</h1>", want: "# Title\n\n"},
		{name: "h2", input: "<h2>Subtitle</h2>", want: "## Subtitle\n\n"},
		{name: "h3", input: "<h3>Section</h3>", want: "### Section\n\n"},
		{name: "h4", input: "<h4>Sub</h4>", want: "#### Sub\n\n"},
		{name: "h5", input: "<h5>Deep</h5>", want: "##### Deep\n\n"},
		{name: "h6", input: "<h6>Deepest</h6>", want: "###### Deepest\n\n"},
		{name: "bold", input: "<p><strong>bold</strong></p>", want: "**bold**\n\n"},
		{name: "italic", input: "<p><em>italic</em></p>", want: "*italic*\n\n"},
		{name: "inline code", input: "<p><code>code</code></p>", want: "`code`\n\n"},
		{name: "link", input: `<p><a href="https://example.com">click</a></p>`, want: "[click](https://example.com)\n\n"},
		{name: "mixed inline", input: "<p>Hello <strong>bold</strong> and <em>italic</em></p>", want: "Hello **bold** and *italic*\n\n"},
		{
			name:  "unordered list",
			input: "<ul><li>One</li><li>Two</li></ul>",
			want:  "- One\n- Two\n\n",
		},
		{
			name:  "ordered list",
			input: "<ol><li>First</li><li>Second</li></ol>",
			want:  "1. First\n2. Second\n\n",
		},
		{
			name:  "nested list",
			input: "<ul><li>Top<ul><li>Nested</li></ul></li></ul>",
			want:  "- Top\n  - Nested\n\n",
		},
		{
			// Confluence storage format wraps <li> content in <p> tags.
			name:  "list with p-wrapped items (Confluence style)",
			input: "<ul><li><p>First</p></li><li><p>Second</p></li></ul>",
			want:  "- First\n- Second\n\n",
		},
		{
			// Ordered list with Confluence-style <p> wrapping.
			name:  "ordered list with p-wrapped items (Confluence style)",
			input: "<ol><li><p>Alpha</p></li><li><p>Beta</p></li></ol>",
			want:  "1. Alpha\n2. Beta\n\n",
		},
		{
			// Nested list where outer item uses <p> wrapping.
			name:  "nested list with p-wrapped parent (Confluence style)",
			input: "<ul><li><p>Top</p><ul><li>Nested</li></ul></li></ul>",
			want:  "- Top\n  - Nested\n\n",
		},
		{
			name:  "pre code block",
			input: `<pre><code>fmt.Println("hello")</code></pre>`,
			want:  "```\nfmt.Println(\"hello\")\n```\n\n",
		},
		{
			name:  "confluence code macro",
			input: `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[func main() {}]]></ac:plain-text-body></ac:structured-macro>`,
			want:  "```go\nfunc main() {}\n```\n\n",
		},
		{
			name:  "confluence info panel",
			input: `<ac:structured-macro ac:name="info"><ac:rich-text-body><p>Note text</p></ac:rich-text-body></ac:structured-macro>`,
			want:  "> **Info:** Note text\n\n",
		},
		{
			name:  "confluence warning panel",
			input: `<ac:structured-macro ac:name="warning"><ac:rich-text-body><p>Danger!</p></ac:rich-text-body></ac:structured-macro>`,
			want:  "> **Warning:** Danger!\n\n",
		},
		{
			name:  "confluence note panel",
			input: `<ac:structured-macro ac:name="note"><ac:rich-text-body><p>Remember this</p></ac:rich-text-body></ac:structured-macro>`,
			want:  "> **Note:** Remember this\n\n",
		},
		{name: "br becomes newline", input: "<p>Line one<br/>Line two</p>", want: "Line one\nLine two\n\n"},
		{name: "hr becomes divider", input: "<hr/>", want: "---\n\n"},
		{
			name:  "simple table",
			input: "<table><tr><th>Name</th><th>Age</th></tr><tr><td>Alice</td><td>30</td></tr></table>",
			want:  "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n\n",
		},
		{
			name:  "strikethrough",
			input: "<p><del>removed</del></p>",
			want:  "~~removed~~\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromHTML(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
