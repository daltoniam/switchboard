// Package markdown provides shared Markdown rendering utilities for Switchboard
// integrations. Document-oriented tools use these to convert API responses
// (HTML, ADF, block trees) into LLM-readable Markdown.
package markdown

import (
	"fmt"
	"strings"
)

// Markdown is rendered Markdown content.
// Semantic type prevents mixing with JSON strings or raw HTML in the response pipeline.
type Markdown string

// NoComments is the standard empty-comments response used by all adapters.
const NoComments = Markdown("No comments.\n")

// Builder provides a fluent API for assembling Markdown documents.
// Only methods with production consumers are included (fewest elements).
type Builder struct {
	sb strings.Builder
}

// NewBuilder returns a new empty builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// ── Block-level methods ─────────────────────────────────────────────

// Metadata writes an HTML comment metadata header.
//
//	b.Metadata("notion", "page_id", "abc", "version", "3")
//	→ <!-- notion:page_id=abc version=3 -->
//
// Panics if kvs has odd length (must be key-value pairs).
func (b *Builder) Metadata(integration string, kvs ...string) {
	if len(kvs)%2 != 0 {
		panic("Metadata: kvs must be key-value pairs (even length)")
	}
	b.sb.WriteString("<!-- " + integration + ":")
	for i := 0; i < len(kvs); i += 2 {
		if i > 0 {
			b.sb.WriteString(" ")
		}
		b.sb.WriteString(kvs[i] + "=" + kvs[i+1])
	}
	b.sb.WriteString(" -->\n")
}

// Heading writes a markdown heading at the given level (1-6).
func (b *Builder) Heading(level int, text string) {
	level = max(1, min(6, level))
	b.sb.WriteString(strings.Repeat("#", level) + " " + text + "\n")
}

// Attribution writes an italic attribution line (e.g. *Author: X | Status: Y*).
func (b *Builder) Attribution(parts ...string) {
	b.sb.WriteString("*" + strings.Join(parts, " | ") + "*\n")
}

// BlockquoteAttribution writes a blockquoted comment with bold author and timestamp.
func (b *Builder) BlockquoteAttribution(author, timestamp, text string) {
	fmt.Fprintf(&b.sb, "> **%s** (%s):\n> %s\n\n", author, timestamp, text)
}

// CommentAttribution writes a non-blockquoted comment with bold author and context.
func (b *Builder) CommentAttribution(author, context, body string) {
	fmt.Fprintf(&b.sb, "**%s** (%s):\n%s\n\n", author, context, body)
}

// Divider writes a horizontal rule.
func (b *Builder) Divider() {
	b.sb.WriteString("---\n")
}

// BlankLine writes an empty line (block separator).
func (b *Builder) BlankLine() {
	b.sb.WriteString("\n")
}

// Raw writes a plain string directly without any markdown formatting.
func (b *Builder) Raw(text string) {
	b.sb.WriteString(text)
}

// WriteMarkdown appends pre-rendered Markdown content to the builder.
func (b *Builder) WriteMarkdown(md Markdown) {
	b.sb.WriteString(string(md))
}

// ── Output ──────────────────────────────────────────────────────────

// Build returns the accumulated content as a typed Markdown value.
func (b *Builder) Build() Markdown {
	return Markdown(b.sb.String())
}

// ── Shared rendering helpers ────────────────────────────────────────

// WriteTable writes a markdown table to sb. First row is the header.
func WriteTable(sb *strings.Builder, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	sb.WriteString("| " + strings.Join(rows[0], " | ") + " |\n")
	sep := make([]string, len(rows[0]))
	for i := range sep {
		sep[i] = "---"
	}
	sb.WriteString("| " + strings.Join(sep, " | ") + " |\n")

	for _, row := range rows[1:] {
		sb.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
}

// ApplyMarks wraps text with the appropriate markdown formatting based on flags.
// Priority: link > code > bold+italic > bold > italic > strike > plain.
func ApplyMarks(sb *strings.Builder, text string, bold, italic, code, strike bool, linkURL string) {
	if linkURL != "" {
		fmt.Fprintf(sb, "[%s](%s)", text, linkURL)
		return
	}
	switch {
	case code:
		sb.WriteString("`")
		sb.WriteString(text)
		sb.WriteString("`")
	case bold && italic:
		sb.WriteString("***")
		sb.WriteString(text)
		sb.WriteString("***")
	case bold:
		sb.WriteString("**")
		sb.WriteString(text)
		sb.WriteString("**")
	case italic:
		sb.WriteString("*")
		sb.WriteString(text)
		sb.WriteString("*")
	case strike:
		sb.WriteString("~~")
		sb.WriteString(text)
		sb.WriteString("~~")
	default:
		sb.WriteString(text)
	}
}
