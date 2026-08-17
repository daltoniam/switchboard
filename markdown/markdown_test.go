package markdown

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownBuilder_Metadata(t *testing.T) {
	b := NewBuilder()
	b.Metadata("notion", "page_id", "abc", "version", "3")
	assert.Equal(t, "<!-- notion:page_id=abc version=3 -->\n", string(b.Build()))
}

func TestMarkdownBuilder_MetadataPanicsOnOddArgs(t *testing.T) {
	b := NewBuilder()
	assert.Panics(t, func() { b.Metadata("test", "key_only") }) //nolint:staticcheck // intentional odd-length to test panic
}

func TestMarkdownBuilder_Heading(t *testing.T) {
	tests := []struct {
		level int
		text  string
		want  string
	}{
		{1, "Title", "# Title\n"},
		{2, "Sub", "## Sub\n"},
		{3, "SubSub", "### SubSub\n"},
		{6, "Deep", "###### Deep\n"},
		{0, "Clamped low", "# Clamped low\n"},
		{9, "Clamped high", "###### Clamped high\n"},
	}
	for _, tt := range tests {
		b := NewBuilder()
		b.Heading(tt.level, tt.text)
		assert.Equal(t, tt.want, string(b.Build()))
	}
}

func TestMarkdownBuilder_Attribution(t *testing.T) {
	b := NewBuilder()
	b.Attribution("Author: Alice", "Status: Open")
	assert.Equal(t, "*Author: Alice | Status: Open*\n", string(b.Build()))
}

func TestMarkdownBuilder_BlockquoteAttribution(t *testing.T) {
	b := NewBuilder()
	b.BlockquoteAttribution("alice", "2024-01-15", "Looks good")
	assert.Equal(t, "> **alice** (2024-01-15):\n> Looks good\n\n", string(b.Build()))
}

func TestMarkdownBuilder_CommentAttribution(t *testing.T) {
	b := NewBuilder()
	b.CommentAttribution("alice", "v2, 2024-01-14", "Updated goals.")
	assert.Equal(t, "**alice** (v2, 2024-01-14):\nUpdated goals.\n\n", string(b.Build()))
}

func TestMarkdownBuilder_Divider(t *testing.T) {
	b := NewBuilder()
	b.Divider()
	assert.Equal(t, "---\n", string(b.Build()))
}

func TestMarkdownBuilder_Composition(t *testing.T) {
	b := NewBuilder()
	b.Metadata("jira", "key", "PROJ-123")
	b.Heading(1, "PROJ-123: Fix auth")
	b.Attribution("Status: Open", "Assignee: Alice")
	b.BlankLine()
	b.Raw("The auth service times out.\n\n")

	got := string(b.Build())
	assert.Contains(t, got, "<!-- jira:key=PROJ-123 -->\n")
	assert.Contains(t, got, "# PROJ-123: Fix auth\n")
	assert.Contains(t, got, "*Status: Open | Assignee: Alice*\n")
	assert.Contains(t, got, "The auth service times out.\n\n")
}

func TestWriteTable(t *testing.T) {
	var sb strings.Builder
	WriteTable(&sb, [][]string{
		{"Name", "Age"},
		{"Alice", "30"},
		{"Bob", "25"},
	})
	assert.Equal(t, "| Name | Age |\n| --- | --- |\n| Alice | 30 |\n| Bob | 25 |\n", sb.String())
}

func TestWriteTable_Empty(t *testing.T) {
	var sb strings.Builder
	WriteTable(&sb, nil)
	assert.Equal(t, "", sb.String())
}

func TestApplyMarks(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		bold   bool
		italic bool
		code   bool
		strike bool
		link   string
		want   string
	}{
		{"plain", "hi", false, false, false, false, "", "hi"},
		{"bold", "hi", true, false, false, false, "", "**hi**"},
		{"italic", "hi", false, true, false, false, "", "*hi*"},
		{"code", "hi", false, false, true, false, "", "`hi`"},
		{"strike", "hi", false, false, false, true, "", "~~hi~~"},
		{"link", "click", false, false, false, false, "https://x.com", "[click](https://x.com)"},
		{"bold+italic", "hi", true, true, false, false, "", "***hi***"},
		{"link wins over bold", "hi", true, false, false, false, "https://x.com", "[hi](https://x.com)"},
		{"code wins over bold", "hi", true, false, true, false, "", "`hi`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			ApplyMarks(&sb, tt.text, tt.bold, tt.italic, tt.code, tt.strike, tt.link)
			assert.Equal(t, tt.want, sb.String())
		})
	}
}
