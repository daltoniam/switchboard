package gslides

import (
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Parity ─────────────────────────────────────────────────────────

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	// Every renderer in the map must have a matching dispatch entry.
	for name := range markdownRenderers {
		_, ok := dispatch[name]
		assert.True(t, ok, "markdown renderer %s has no dispatch handler", name)
	}
}

func TestRenderMarkdown_UnknownTool(t *testing.T) {
	g := &gslides{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_nope"), []byte(`{}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── get_presentation ───────────────────────────────────────────────

func TestRenderPresentation_Basic(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "PRES_ABC",
		"title": "Quarterly Review",
		"locale": "en",
		"revisionId": "rev42",
		"slides": [
			{"objectId": "slide1", "pageElements": []},
			{"objectId": "slide2", "pageElements": []}
		]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "presentation_id=PRES_ABC")
	assert.Contains(t, s, "# Quarterly Review")
	assert.Contains(t, s, "Locale: en")
	assert.Contains(t, s, "Revision: rev42")
	assert.Contains(t, s, "## Slides (2)")
	assert.Contains(t, s, "Slide 1 (`slide1`)")
	assert.Contains(t, s, "Slide 2 (`slide2`)")
}

func TestRenderPresentation_UntitledFallback(t *testing.T) {
	g := &gslides{}
	body := []byte(`{"presentationId":"P","slides":[]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "# (untitled presentation)")
}

func TestRenderPresentation_TitlePlaceholder(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"shape": {
					"placeholder": {"type": "TITLE"},
					"text": {"textElements": [
						{"paragraphMarker": {}},
						{"textRun": {"content": "Welcome\n"}}
					]}
				}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	s := string(md)
	// TITLE placeholder is promoted to a level-4 heading.
	assert.Contains(t, s, "#### Welcome")
}

func TestRenderPresentation_Bullets(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"shape": {
					"text": {"textElements": [
						{"paragraphMarker": {"bullet": {"nestingLevel": 0}}},
						{"textRun": {"content": "First\n"}},
						{"paragraphMarker": {"bullet": {"nestingLevel": 1}}},
						{"textRun": {"content": "Nested\n"}}
					]}
				}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "- First")
	assert.Contains(t, s, "  - Nested")
}

func TestRenderPresentation_WithLinks(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"shape": {
					"text": {"textElements": [
						{"paragraphMarker": {}},
						{"textRun": {"content": "Docs", "style": {"link": {"url": "https://example.com"}}}},
						{"textRun": {"content": "\n"}}
					]}
				}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "[Docs](https://example.com)")
}

func TestRenderPresentation_WithTable(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"table": {
					"rows": 2,
					"columns": 2,
					"tableRows": [
						{"tableCells": [
							{"text": {"textElements": [{"paragraphMarker": {}}, {"textRun": {"content": "Q1\n"}}]}},
							{"text": {"textElements": [{"paragraphMarker": {}}, {"textRun": {"content": "Q2\n"}}]}}
						]},
						{"tableCells": [
							{"text": {"textElements": [{"paragraphMarker": {}}, {"textRun": {"content": "100\n"}}]}},
							{"text": {"textElements": [{"paragraphMarker": {}}, {"textRun": {"content": "200\n"}}]}}
						]}
					]
				}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "| Q1 | Q2 |")
	assert.Contains(t, s, "| 100 | 200 |")
}

func TestRenderPresentation_TableBlankHeader(t *testing.T) {
	g := &gslides{}
	// First row blank → synthesize A,B column labels.
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"table": {
					"rows": 2,
					"columns": 2,
					"tableRows": [
						{"tableCells": [
							{"text": {"textElements": []}},
							{"text": {"textElements": []}}
						]},
						{"tableCells": [
							{"text": {"textElements": [{"paragraphMarker": {}}, {"textRun": {"content": "x\n"}}]}},
							{"text": {"textElements": [{"paragraphMarker": {}}, {"textRun": {"content": "y\n"}}]}}
						]}
					]
				}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "| A | B |")
}

func TestRenderPresentation_WithImage(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"title": "Logo",
				"image": {"sourceUrl": "https://example.com/logo.png"}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "![Logo](https://example.com/logo.png)")
}

func TestRenderPresentation_WithImage_ContentURLFallback(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"image": {"contentUrl": "https://cdn.example.com/img.png"}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	s := string(md)
	// No title/description → alt falls back to "image".
	assert.Contains(t, s, "![image](https://cdn.example.com/img.png)")
}

func TestRenderPresentation_WithVideo(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{
				"video": {"url": "https://youtu.be/abc", "source": "YOUTUBE"}
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "📺 [Video: YOUTUBE](https://youtu.be/abc)")
}

func TestRenderPresentation_WithWordArt(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{"wordArt": {"renderedText": "BIG"}}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "**BIG**")
}

func TestRenderPresentation_WithSheetsChart(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"presentationId": "P",
		"title": "Deck",
		"slides": [{
			"objectId": "s1",
			"pageElements": [{"sheetsChart": {"spreadsheetId": "SHEET_X", "chartId": 99}}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Sheets chart")
	assert.Contains(t, s, "SHEET_X")
	assert.Contains(t, s, "chart_id=99")
}

func TestRenderPresentation_InvalidJSON(t *testing.T) {
	g := &gslides{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), []byte(`{not json`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

func TestRenderPresentation_MissingID(t *testing.T) {
	g := &gslides{}
	// No presentationId → fall back to raw JSON.
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_presentation"), []byte(`{"title":"x"}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── get_page ───────────────────────────────────────────────────────

func TestRenderPage_Basic(t *testing.T) {
	g := &gslides{}
	body := []byte(`{
		"objectId": "slideZ",
		"pageElements": [{
			"shape": {
				"text": {"textElements": [
					{"paragraphMarker": {}},
					{"textRun": {"content": "Body text\n"}}
				]}
			}
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_page"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "page_object_id=slideZ")
	assert.Contains(t, s, "Slide 1 (`slideZ`)")
	assert.Contains(t, s, "Body text")
}

func TestRenderPage_MissingID(t *testing.T) {
	g := &gslides{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_page"), []byte(`{"pageElements":[]}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

func TestRenderPage_InvalidJSON(t *testing.T) {
	g := &gslides{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gslides_get_page"), []byte(`xxx`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── Helpers ────────────────────────────────────────────────────────

func TestPipeSafe_Newlines(t *testing.T) {
	assert.Equal(t, "a b c", pipeSafe("a\nb\nc"))
	assert.Equal(t, `a\|b`, pipeSafe("a|b"))
}

func TestColLabel(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "A"}, {1, "B"}, {25, "Z"},
		{26, "AA"}, {27, "AB"}, {51, "AZ"}, {52, "BA"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, colLabel(c.idx), "idx=%d", c.idx)
	}
}

func TestIsBlankRow(t *testing.T) {
	assert.True(t, isBlankRow([]string{"", "", ""}))
	assert.False(t, isBlankRow([]string{"", "x", ""}))
}

func TestRenderTextBox_NilAndEmpty(t *testing.T) {
	text, isTitle := renderTextBox(nil, nil)
	assert.Equal(t, "", text)
	assert.False(t, isTitle)

	text, isTitle = renderTextBox(&rawTextBox{}, &rawPlace{Type: "CENTERED_TITLE"})
	assert.Equal(t, "", text)
	assert.True(t, isTitle)
}

func TestRenderTextBox_StripsTrailingNewlines(t *testing.T) {
	tb := &rawTextBox{TextElements: []rawTextElement{
		{ParagraphMarker: &rawParagraphMarker{}},
		{TextRun: &rawTextRun{Content: "hello\n"}},
	}}
	text, _ := renderTextBox(tb, nil)
	assert.False(t, strings.HasSuffix(text, "\n"), "expected no trailing newline, got %q", text)
	assert.Equal(t, "hello", text)
}
