package gslides

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gslides_get_presentation": renderPresentationMD,
	"gslides_get_page":         renderPageMD,
}

func (g *gslides) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw types ───────────────────────────────────────────────────────

type rawPresentation struct {
	PresentationID string    `json:"presentationId"`
	Title          string    `json:"title"`
	RevisionID     string    `json:"revisionId"`
	Locale         string    `json:"locale"`
	Slides         []rawPage `json:"slides"`
}

type rawPage struct {
	ObjectID     string           `json:"objectId"`
	PageType     string           `json:"pageType"`
	PageElements []rawPageElement `json:"pageElements"`
	SlideProps   *rawSlideProps   `json:"slideProperties,omitempty"`
}

type rawSlideProps struct {
	LayoutObjectID string `json:"layoutObjectId"`
	MasterObjectID string `json:"masterObjectId"`
}

type rawPageElement struct {
	ObjectID    string    `json:"objectId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Shape       *rawShape `json:"shape,omitempty"`
	Table       *rawTable `json:"table,omitempty"`
	Image       *rawImage `json:"image,omitempty"`
	Video       *rawVideo `json:"video,omitempty"`
	WordArt     *rawWord  `json:"wordArt,omitempty"`
	SheetsChart *rawChart `json:"sheetsChart,omitempty"`
}

type rawShape struct {
	ShapeType   string      `json:"shapeType"`
	Placeholder *rawPlace   `json:"placeholder,omitempty"`
	Text        *rawTextBox `json:"text,omitempty"`
}

type rawPlace struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

type rawTextBox struct {
	TextElements []rawTextElement `json:"textElements"`
}

type rawTextElement struct {
	TextRun         *rawTextRun         `json:"textRun,omitempty"`
	ParagraphMarker *rawParagraphMarker `json:"paragraphMarker,omitempty"`
}

type rawTextRun struct {
	Content string        `json:"content"`
	Style   *rawTextStyle `json:"style,omitempty"`
}

type rawTextStyle struct {
	Link *rawLink `json:"link,omitempty"`
}

type rawLink struct {
	URL string `json:"url"`
}

type rawParagraphMarker struct {
	Bullet *rawBullet    `json:"bullet,omitempty"`
	Style  *rawParaStyle `json:"style,omitempty"`
}

type rawBullet struct {
	ListID       string `json:"listId"`
	NestingLevel int    `json:"nestingLevel"`
	Glyph        string `json:"glyph"`
}

type rawParaStyle struct {
	NamedStyleType string `json:"namedStyleType"`
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
	Text *rawTextBox `json:"text,omitempty"`
}

type rawImage struct {
	ContentURL string `json:"contentUrl"`
	SourceURL  string `json:"sourceUrl"`
}

type rawVideo struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

type rawWord struct {
	RenderedText string `json:"renderedText"`
}

type rawChart struct {
	SpreadsheetID string `json:"spreadsheetId"`
	ChartID       int64  `json:"chartId"`
	ContentURL    string `json:"contentUrl"`
}

// ── Rendering ───────────────────────────────────────────────────────

func renderPresentationMD(data []byte) (markdown.Markdown, bool) {
	var p rawPresentation
	if err := json.Unmarshal(data, &p); err != nil {
		return "", false
	}
	if p.PresentationID == "" {
		// Not a presentation resource (e.g. an error envelope) — bail
		// out so the framework falls back to JSON.
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gslides", "presentation_id", p.PresentationID, "slides", fmt.Sprintf("%d", len(p.Slides)))

	title := p.Title
	if title == "" {
		title = "(untitled presentation)"
	}
	b.Heading(1, title)
	if p.Locale != "" || p.RevisionID != "" {
		attrs := []string{}
		if p.Locale != "" {
			attrs = append(attrs, "Locale: "+p.Locale)
		}
		if p.RevisionID != "" {
			attrs = append(attrs, "Revision: "+p.RevisionID)
		}
		b.Attribution(attrs...)
	}
	b.BlankLine()

	b.Heading(2, fmt.Sprintf("Slides (%d)", len(p.Slides)))
	b.BlankLine()
	for i, slide := range p.Slides {
		renderSlideSection(b, i+1, &slide)
	}
	return b.Build(), true
}

func renderPageMD(data []byte) (markdown.Markdown, bool) {
	var page rawPage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", false
	}
	if page.ObjectID == "" {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gslides", "page_object_id", page.ObjectID)
	renderSlideSection(b, 1, &page)
	return b.Build(), true
}

func renderSlideSection(b *markdown.Builder, slideNum int, slide *rawPage) {
	heading := fmt.Sprintf("Slide %d", slideNum)
	if slide.ObjectID != "" {
		heading = fmt.Sprintf("Slide %d (`%s`)", slideNum, slide.ObjectID)
	}
	b.Heading(3, heading)
	attrs := []string{}
	if slide.PageType != "" && slide.PageType != "SLIDE" {
		attrs = append(attrs, "type="+slide.PageType)
	}
	if len(slide.PageElements) > 0 {
		attrs = append(attrs, fmt.Sprintf("%d elements", len(slide.PageElements)))
	}
	if len(attrs) > 0 {
		b.Attribution(attrs...)
	}
	b.BlankLine()

	for _, el := range slide.PageElements {
		renderPageElement(b, &el)
	}
	b.BlankLine()
}

func renderPageElement(b *markdown.Builder, el *rawPageElement) {
	switch {
	case el.Shape != nil && el.Shape.Text != nil:
		text, isTitle := renderTextBox(el.Shape.Text, el.Shape.Placeholder)
		if text == "" {
			return
		}
		if isTitle {
			// Render placeholder titles as a level-4 heading so the
			// outline mirrors the deck structure.
			b.Heading(4, text)
		} else {
			b.Raw(text)
			if !strings.HasSuffix(text, "\n") {
				b.Raw("\n")
			}
		}
		b.BlankLine()
	case el.Table != nil:
		renderTable(b, el.Table)
		b.BlankLine()
	case el.Image != nil:
		alt := el.Title
		if alt == "" {
			alt = el.Description
		}
		if alt == "" {
			alt = "image"
		}
		url := el.Image.SourceURL
		if url == "" {
			url = el.Image.ContentURL
		}
		if url != "" {
			b.Raw(fmt.Sprintf("![%s](%s)\n", pipeSafe(alt), url))
			b.BlankLine()
		}
	case el.Video != nil:
		if el.Video.URL != "" {
			b.Raw(fmt.Sprintf("📺 [Video: %s](%s)\n", el.Video.Source, el.Video.URL))
			b.BlankLine()
		}
	case el.WordArt != nil:
		if el.WordArt.RenderedText != "" {
			b.Raw("**" + el.WordArt.RenderedText + "**\n")
			b.BlankLine()
		}
	case el.SheetsChart != nil:
		if el.SheetsChart.SpreadsheetID != "" {
			b.Raw(fmt.Sprintf("📊 _Sheets chart_ (`%s`, chart_id=%d)\n", el.SheetsChart.SpreadsheetID, el.SheetsChart.ChartID))
			b.BlankLine()
		}
	}
}

// renderTextBox flattens a Slides textBox into plain markdown text.
// Paragraph markers terminate the current paragraph; bullets become
// list items at the appropriate nesting level. Returns (text, isTitle)
// where isTitle is true when the placeholder is TITLE/CENTERED_TITLE.
func renderTextBox(tb *rawTextBox, ph *rawPlace) (string, bool) {
	isTitle := ph != nil && (ph.Type == "TITLE" || ph.Type == "CENTERED_TITLE")
	if tb == nil || len(tb.TextElements) == 0 {
		return "", isTitle
	}

	var sb strings.Builder
	// Currently-open bullet (if any).
	var pendingBullet *rawBullet
	for _, te := range tb.TextElements {
		if te.ParagraphMarker != nil {
			pendingBullet = te.ParagraphMarker.Bullet
			continue
		}
		if te.TextRun == nil {
			continue
		}
		content := te.TextRun.Content
		if content == "" {
			continue
		}
		// Strip the trailing newline embedded in the run; we'll
		// re-add it explicitly after rendering bullets/links so the
		// output is deterministic.
		hasNL := strings.HasSuffix(content, "\n")
		content = strings.TrimSuffix(content, "\n")

		if pendingBullet != nil && !isTitle {
			indent := strings.Repeat("  ", pendingBullet.NestingLevel)
			sb.WriteString(indent + "- ")
		}

		if te.TextRun.Style != nil && te.TextRun.Style.Link != nil && te.TextRun.Style.Link.URL != "" {
			sb.WriteString("[" + content + "](" + te.TextRun.Style.Link.URL + ")")
		} else {
			sb.WriteString(content)
		}

		if hasNL {
			sb.WriteString("\n")
			pendingBullet = nil
		}
	}
	return strings.TrimRight(sb.String(), "\n"), isTitle
}

func renderTable(b *markdown.Builder, t *rawTable) {
	if t == nil || len(t.TableRows) == 0 {
		return
	}
	cols := t.Columns
	if cols == 0 {
		for _, row := range t.TableRows {
			if n := len(row.TableCells); n > cols {
				cols = n
			}
		}
	}
	if cols == 0 {
		return
	}
	rows := make([][]string, 0, len(t.TableRows))
	for _, row := range t.TableRows {
		rendered := make([]string, cols)
		for i := 0; i < cols; i++ {
			if i < len(row.TableCells) && row.TableCells[i].Text != nil {
				txt, _ := renderTextBox(row.TableCells[i].Text, nil)
				rendered[i] = pipeSafe(txt)
			}
		}
		rows = append(rows, rendered)
	}
	if isBlankRow(rows[0]) {
		header := make([]string, cols)
		for i := 0; i < cols; i++ {
			header[i] = colLabel(i)
		}
		rows = append([][]string{header}, rows...)
	}
	var sb strings.Builder
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())
}

func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

func isBlankRow(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}
	return true
}

// colLabel returns a 1-based spreadsheet column label (A, B, ..., Z, AA, AB).
func colLabel(idx int) string {
	idx++
	var sb strings.Builder
	for idx > 0 {
		idx--
		sb.WriteByte(byte('A' + idx%26))
		idx /= 26
	}
	b := sb.String()
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return string(out)
}
