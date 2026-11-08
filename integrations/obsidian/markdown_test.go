package obsidian

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	adapter := New()
	_, ok := adapter.(mcp.MarkdownIntegration)
	require.True(t, ok, "adapter should implement MarkdownIntegration")

	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range adapter.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range markdownRenderers {
		assert.True(t, toolNames[name], "RenderMarkdown handles %q but it's not in Tools()", name)
	}
}

func TestRenderMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		tool    mcp.ToolName
		data    string
		ok      bool
		want    []string
		notWant []string
	}{
		{
			name: "full note", tool: "obsidian_read_note", data: noteFixture, ok: true,
			want:    []string{"<!-- obsidian:path=Projects/Plan A.md", "modified=2023-11-14T22:", "*Tags: #work, #work/plans*", "*Links: Ideas.md*", "status: draft", "# Plan\n\nShip it.\n"},
			notWant: []string{`"frontmatter"`, "ctime"},
		},
		{
			name: "active note", tool: "obsidian_get_active_note", data: noteFixture, ok: true,
			want: []string{"<!-- obsidian:path=Projects/Plan A.md", "Ship it."},
		},
		{
			name: "targeted section", tool: "obsidian_read_note", data: `{"path":"a.md","target":"heading: Plan","content":"Ship it.\n"}`, ok: true,
			want: []string{"<!-- obsidian:path=a.md", "*Section: heading: Plan*", "Ship it.\n"},
		},
		{name: "not a note", tool: "obsidian_read_note", data: `{}`, ok: false},
		{name: "bad json", tool: "obsidian_read_note", data: `nope`, ok: false},
		{name: "json tool", tool: "obsidian_search", data: `{}`, ok: false},
	}
	o := New().(*obsidian)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, ok := o.RenderMarkdown(tt.tool, []byte(tt.data))
			require.Equal(t, tt.ok, ok)
			for _, w := range tt.want {
				assert.Contains(t, string(md), w)
			}
			for _, w := range tt.notWant {
				assert.NotContains(t, string(md), w)
			}
		})
	}
}
