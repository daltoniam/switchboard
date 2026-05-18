package gdocs

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gdocs_get_document": renderDocumentMD,
}

func (g *gdocs) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw JSON parse types ────────────────────────────────────────────

type rawDocument struct {
	DocumentID string                  `json:"documentId"`
	Title      string                  `json:"title"`
	RevisionID string                  `json:"revisionId"`
	Body       rawDocumentBody         `json:"body"`
	Lists      map[string]rawListEntry `json:"lists"`
}

type rawDocumentBody struct {
	Content []rawStructuralElement `json:"content"`
}

type rawStructuralElement struct {
	StartIndex      int                 `json:"startIndex"`
	EndIndex        int                 `json:"endIndex"`
	Paragraph       *rawParagraph       `json:"paragraph,omitempty"`
	SectionBreak    *rawSectionBreak    `json:"sectionBreak,omitempty"`
	Table           *rawTable           `json:"table,omitempty"`
	TableOfContents *rawTableOfContents `json:"tableOfContents,omitempty"`
}

type rawSectionBreak struct {
	// Presence is enough; we don't render its style.
}

type rawParagraph struct {
	Elements       []rawParagraphElement `json:"elements"`
	ParagraphStyle rawParagraphStyle     `json:"paragraphStyle"`
	Bullet         *rawBullet            `json:"bullet,omitempty"`
}

type rawParagraphStyle struct {
	NamedStyleType string `json:"namedStyleType"`
}

type rawBullet struct {
	ListID       string `json:"listId"`
	NestingLevel int    `json:"nestingLevel"`
}

type rawParagraphElement struct {
	TextRun             *rawTextRun             `json:"textRun,omitempty"`
	InlineObjectElement *rawInlineObjectElement `json:"inlineObjectElement,omitempty"`
	PageBreak           *rawPageBreak           `json:"pageBreak,omitempty"`
	HorizontalRule      *rawHorizontalRule      `json:"horizontalRule,omitempty"`
	FootnoteReference   *rawFootnoteReference   `json:"footnoteReference,omitempty"`
}

type rawTextRun struct {
	Content   string       `json:"content"`
	TextStyle rawTextStyle `json:"textStyle"`
}

type rawTextStyle struct {
	Bold          bool    `json:"bold"`
	Italic        bool    `json:"italic"`
	Underline     bool    `json:"underline"`
	Strikethrough bool    `json:"strikethrough"`
	Link          *rawURL `json:"link,omitempty"`
}

type rawURL struct {
	URL string `json:"url"`
}

type rawInlineObjectElement struct {
	InlineObjectID string `json:"inlineObjectId"`
}

type rawPageBreak struct{}

type rawHorizontalRule struct{}

type rawFootnoteReference struct {
	FootnoteID     string `json:"footnoteId"`
	FootnoteNumber string `json:"footnoteNumber"`
}

type rawTable struct {
	Rows      int           `json:"rows"`
	Columns   int           `json:"columns"`
	TableRows []rawTableRow `json:"tableRows"`
}

type rawTableRow struct {
	TableCells []rawTableCell `json:"tableCells"`
}

type rawTableCell struct {
	Content []rawStructuralElement `json:"content"`
}

type rawTableOfContents struct {
	Content []rawStructuralElement `json:"content"`
}

type rawListEntry struct {
	ListProperties rawListProperties `json:"listProperties"`
}

type rawListProperties struct {
	NestingLevels []rawNestingLevel `json:"nestingLevels"`
}

type rawNestingLevel struct {
	// glyphType: "DECIMAL", "ALPHA", "ROMAN", "DISC", etc. We use it to
	// distinguish ordered (numeric/alpha/roman) from unordered (bullet)
	// lists.
	GlyphType string `json:"glyphType"`
}

// ── Rendering ───────────────────────────────────────────────────────

func renderDocumentMD(data []byte) (markdown.Markdown, bool) {
	var doc rawDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", false
	}
	if doc.DocumentID == "" {
		// Not a document resource (e.g. an error envelope) — bail out so
		// the framework falls back to JSON.
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gdocs", "document_id", doc.DocumentID, "revision_id", doc.RevisionID)

	title := doc.Title
	if title == "" {
		title = "(untitled document)"
	}
	b.Heading(1, title)
	b.BlankLine()

	renderContent(b, doc.Body.Content, doc.Lists, 0)

	return b.Build(), true
}

// renderContent walks a slice of StructuralElements (the document body, or
// the body of a table cell, or a table-of-contents block) and emits the
// equivalent markdown. depth is used to avoid infinite recursion through
// nested tables of contents.
func renderContent(b *markdown.Builder, content []rawStructuralElement, lists map[string]rawListEntry, depth int) {
	if depth > 6 {
		return
	}
	for _, el := range content {
		switch {
		case el.Paragraph != nil:
			renderParagraph(b, el.Paragraph, lists)
		case el.Table != nil:
			renderTable(b, el.Table, lists, depth)
		case el.SectionBreak != nil:
			// Section breaks become a thematic break for visibility.
			b.Raw("\n---\n\n")
		case el.TableOfContents != nil:
			b.Heading(2, "Table of Contents")
			renderContent(b, el.TableOfContents.Content, lists, depth+1)
		}
	}
}

func renderParagraph(b *markdown.Builder, p *rawParagraph, lists map[string]rawListEntry) {
	// Collect the paragraph's inline text first so we can decide whether
	// to emit it as a heading, a list item, or plain content.
	var sb strings.Builder
	for _, el := range p.Elements {
		switch {
		case el.TextRun != nil:
			sb.WriteString(applyInlineStyle(el.TextRun))
		case el.HorizontalRule != nil:
			sb.WriteString("\n---\n")
		case el.PageBreak != nil:
			// Markdown has no page-break primitive; use a thematic break.
			sb.WriteString("\n---\n")
		case el.InlineObjectElement != nil:
			fmt.Fprintf(&sb, "[inline object %s]", el.InlineObjectElement.InlineObjectID)
		case el.FootnoteReference != nil:
			n := el.FootnoteReference.FootnoteNumber
			if n == "" {
				n = el.FootnoteReference.FootnoteID
			}
			fmt.Fprintf(&sb, "[^%s]", n)
		}
	}
	text := sb.String()
	// Paragraphs typically end with a literal newline character in the
	// Docs API output; trim it so it doesn't fight our own emitter.
	text = strings.TrimRight(text, "\n")

	// Bullet/numbered list?
	if p.Bullet != nil {
		ordered := isOrderedList(p.Bullet.ListID, lists)
		indent := strings.Repeat("  ", p.Bullet.NestingLevel)
		marker := "-"
		if ordered {
			marker = "1."
		}
		if text == "" {
			text = " "
		}
		b.Raw(fmt.Sprintf("%s%s %s\n", indent, marker, text))
		return
	}

	// Heading vs. body text via namedStyleType.
	switch p.ParagraphStyle.NamedStyleType {
	case "TITLE":
		if text != "" {
			b.Heading(1, text)
			b.BlankLine()
		}
	case "SUBTITLE":
		if text != "" {
			b.Heading(2, text)
			b.BlankLine()
		}
	case "HEADING_1":
		if text != "" {
			b.Heading(1, text)
			b.BlankLine()
		}
	case "HEADING_2":
		if text != "" {
			b.Heading(2, text)
			b.BlankLine()
		}
	case "HEADING_3":
		if text != "" {
			b.Heading(3, text)
			b.BlankLine()
		}
	case "HEADING_4":
		if text != "" {
			b.Heading(4, text)
			b.BlankLine()
		}
	case "HEADING_5":
		if text != "" {
			b.Heading(5, text)
			b.BlankLine()
		}
	case "HEADING_6":
		if text != "" {
			b.Heading(6, text)
			b.BlankLine()
		}
	default:
		// NORMAL_TEXT or unspecified — plain paragraph.
		if text == "" {
			// Empty paragraph still emits a blank line for spacing.
			b.BlankLine()
			return
		}
		b.Raw(text + "\n\n")
	}
}

// applyInlineStyle wraps a text-run's content in markdown emphasis markers
// based on its TextStyle. Order is bold > italic > strikethrough; underline
// has no markdown equivalent so we drop it.
func applyInlineStyle(t *rawTextRun) string {
	content := t.Content
	if content == "" {
		return ""
	}
	// Don't apply emphasis around a newline-only run (it just messes up
	// formatting); pass through verbatim.
	if strings.TrimSpace(content) == "" {
		return content
	}
	if t.TextStyle.Link != nil && t.TextStyle.Link.URL != "" {
		content = fmt.Sprintf("[%s](%s)", strings.TrimRight(content, "\n"), t.TextStyle.Link.URL)
	}
	if t.TextStyle.Strikethrough {
		content = "~~" + strings.TrimRight(content, "\n") + "~~"
	}
	if t.TextStyle.Bold && t.TextStyle.Italic {
		content = "***" + strings.TrimRight(content, "\n") + "***"
	} else if t.TextStyle.Bold {
		content = "**" + strings.TrimRight(content, "\n") + "**"
	} else if t.TextStyle.Italic {
		content = "*" + strings.TrimRight(content, "\n") + "*"
	}
	return content
}

// renderTable emits a Markdown pipe table when the cells contain only
// single paragraphs (the common case). For multi-paragraph cells we fall
// back to a flat list of rows.
func renderTable(b *markdown.Builder, t *rawTable, lists map[string]rawListEntry, depth int) {
	if len(t.TableRows) == 0 {
		return
	}
	if !isSimpleTable(t) {
		renderComplexTable(b, t, lists, depth)
		return
	}

	cols := t.Columns
	if cols == 0 && len(t.TableRows) > 0 {
		cols = len(t.TableRows[0].TableCells)
	}

	// Header row uses the first table row.
	header := cellTexts(t.TableRows[0])
	rows := make([][]string, 0, len(t.TableRows)-1)
	for _, r := range t.TableRows[1:] {
		rows = append(rows, cellTexts(r))
	}

	b.BlankLine()
	writeTable(b, header, rows, cols)
	b.BlankLine()
}

func writeTable(b *markdown.Builder, header []string, rows [][]string, cols int) {
	if cols == 0 {
		return
	}
	for len(header) < cols {
		header = append(header, "")
	}
	b.Raw("| " + strings.Join(header[:cols], " | ") + " |\n")
	sep := make([]string, cols)
	for i := range sep {
		sep[i] = "---"
	}
	b.Raw("| " + strings.Join(sep, " | ") + " |\n")
	for _, r := range rows {
		for len(r) < cols {
			r = append(r, "")
		}
		b.Raw("| " + strings.Join(r[:cols], " | ") + " |\n")
	}
}

func renderComplexTable(b *markdown.Builder, t *rawTable, lists map[string]rawListEntry, depth int) {
	b.Heading(4, fmt.Sprintf("Table (%d rows × %d cols)", len(t.TableRows), t.Columns))
	for ri, row := range t.TableRows {
		for ci, cell := range row.TableCells {
			b.Raw(fmt.Sprintf("**Row %d, Col %d:**\n\n", ri+1, ci+1))
			renderContent(b, cell.Content, lists, depth+1)
		}
	}
}

func isSimpleTable(t *rawTable) bool {
	for _, row := range t.TableRows {
		for _, cell := range row.TableCells {
			if len(cell.Content) != 1 {
				return false
			}
			el := cell.Content[0]
			if el.Paragraph == nil {
				return false
			}
			if el.Table != nil || el.TableOfContents != nil {
				return false
			}
		}
	}
	return true
}

// cellTexts collapses a row's simple cells to a slice of pipe-safe strings.
func cellTexts(row rawTableRow) []string {
	out := make([]string, 0, len(row.TableCells))
	for _, cell := range row.TableCells {
		var s strings.Builder
		for _, el := range cell.Content {
			if el.Paragraph == nil {
				continue
			}
			for _, pe := range el.Paragraph.Elements {
				if pe.TextRun != nil {
					s.WriteString(pe.TextRun.Content)
				}
			}
		}
		out = append(out, pipeSafe(strings.TrimSpace(s.String())))
	}
	return out
}

// pipeSafe escapes characters that would break a Markdown table row.
func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

func isOrderedList(listID string, lists map[string]rawListEntry) bool {
	if listID == "" {
		return false
	}
	entry, ok := lists[listID]
	if !ok || len(entry.ListProperties.NestingLevels) == 0 {
		return false
	}
	gt := entry.ListProperties.NestingLevels[0].GlyphType
	switch gt {
	case "DECIMAL", "ZERO_DECIMAL", "ALPHA", "UPPER_ALPHA", "ROMAN", "UPPER_ROMAN":
		return true
	}
	return false
}
