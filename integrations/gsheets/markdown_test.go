package gsheets

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
	g := &gsheets{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_nope"), []byte(`{}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── get_values ─────────────────────────────────────────────────────

func TestRenderGetValues_Basic(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{
		"range": "Sheet1!A1:C3",
		"majorDimension": "ROWS",
		"values": [
			["Name", "Age", "City"],
			["Alice", 30, "NYC"],
			["Bob", 25, "SF"]
		]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Range: Sheet1!A1:C3")
	assert.Contains(t, s, "| Name | Age | City |")
	assert.Contains(t, s, "| Alice | 30 | NYC |")
	assert.Contains(t, s, "| Bob | 25 | SF |")
}

func TestRenderGetValues_Empty(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{"range":"Sheet1!A1:B2","majorDimension":"ROWS"}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "(no values)")
}

func TestRenderGetValues_ColumnsTransposed(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{
		"range": "Sheet1!A1:B3",
		"majorDimension": "COLUMNS",
		"values": [
			["a1","a2","a3"],
			["b1","b2","b3"]
		]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), body)
	require.True(t, ok)
	s := string(md)
	// After transpose we should see row-major output: header row "a1 | b1"
	assert.Contains(t, s, "| a1 | b1 |")
	assert.Contains(t, s, "| a2 | b2 |")
	assert.Contains(t, s, "| a3 | b3 |")
}

func TestRenderGetValues_InvalidJSON(t *testing.T) {
	g := &gsheets{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), []byte("not-json"))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

func TestRenderGetValues_MissingRange(t *testing.T) {
	g := &gsheets{}
	// Valid JSON, but no `range` field — bail out so framework falls
	// back to raw JSON.
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), []byte(`{"foo":"bar"}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

func TestRenderGetValues_PipeEscape(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{"range":"Sheet1","majorDimension":"ROWS","values":[["a|b","c"]]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), `a\|b`)
}

func TestRenderGetValues_NumberFormatting(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{"range":"S","majorDimension":"ROWS","values":[["x"],[42],[1.5]]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), body)
	require.True(t, ok)
	s := string(md)
	// Integer values render without trailing .0.
	assert.Contains(t, s, "| 42 |")
	assert.Contains(t, s, "| 1.5 |")
}

func TestRenderGetValues_BoolFormatting(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{"range":"S","majorDimension":"ROWS","values":[["x"],[true],[false]]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "| TRUE |")
	assert.Contains(t, s, "| FALSE |")
}

// ── batch_get_values ────────────────────────────────────────────────

func TestRenderBatchGetValues(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{
		"spreadsheetId":"s-1",
		"valueRanges":[
			{"range":"Sheet1!A1:B2","majorDimension":"ROWS","values":[["h1","h2"],["v1","v2"]]},
			{"range":"Sheet2!C","majorDimension":"ROWS","values":[["c1"]]}
		]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_batch_get_values"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "Range: Sheet1!A1:B2")
	assert.Contains(t, s, "Range: Sheet2!C")
	assert.Contains(t, s, "| h1 | h2 |")
}

func TestRenderBatchGetValues_Empty(t *testing.T) {
	g := &gsheets{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_batch_get_values"), []byte(`{}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── get_spreadsheet ────────────────────────────────────────────────

func TestRenderSpreadsheet_Basic(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{
		"spreadsheetId":"s-1",
		"spreadsheetUrl":"https://docs.google.com/spreadsheets/d/s-1",
		"properties":{"title":"Budget","locale":"en_US","timeZone":"America/New_York"},
		"sheets":[
			{"properties":{"sheetId":0,"title":"Summary","gridProperties":{"rowCount":100,"columnCount":26}}},
			{"properties":{"sheetId":12345,"title":"Hidden","hidden":true,"gridProperties":{"rowCount":10,"columnCount":5}}}
		]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_spreadsheet"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "# Budget")
	assert.Contains(t, s, "Open in Google Sheets")
	assert.Contains(t, s, "## Sheets (2)")
	assert.Contains(t, s, "### Summary")
	assert.Contains(t, s, "sheet_id=0")
	assert.Contains(t, s, "100r × 26c")
	assert.Contains(t, s, "### Hidden")
	assert.Contains(t, s, "hidden")
}

func TestRenderSpreadsheet_WithGridData(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{
		"spreadsheetId":"s-1",
		"properties":{"title":"Data"},
		"sheets":[{
			"properties":{"sheetId":0,"title":"Data","gridProperties":{"rowCount":2,"columnCount":2}},
			"data":[{
				"startRow":0,"startColumn":0,
				"rowData":[
					{"values":[{"formattedValue":"name"},{"formattedValue":"score"}]},
					{"values":[{"formattedValue":"alice"},{"formattedValue":"42"}]}
				]
			}]
		}]
	}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_spreadsheet"), body)
	require.True(t, ok)
	s := string(md)
	assert.Contains(t, s, "| name | score |")
	assert.Contains(t, s, "| alice | 42 |")
}

func TestRenderSpreadsheet_Untitled(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{"spreadsheetId":"s-1","properties":{},"sheets":[]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_spreadsheet"), body)
	require.True(t, ok)
	assert.Contains(t, string(md), "(untitled spreadsheet)")
}

func TestRenderSpreadsheet_MissingID(t *testing.T) {
	g := &gsheets{}
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_spreadsheet"), []byte(`{"foo":"bar"}`))
	assert.False(t, ok)
	assert.Equal(t, "", string(md))
}

// ── colLabel helper ────────────────────────────────────────────────

func TestColLabel(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{701, "ZZ"},
		{702, "AAA"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			assert.Equal(t, c.want, colLabel(c.idx))
		})
	}
}

// Sanity: pipe-safe should join newlines into spaces.
func TestPipeSafe_Newlines(t *testing.T) {
	assert.Equal(t, "a b", pipeSafe("a\nb"))
	assert.Equal(t, `a\|b`, pipeSafe("a|b"))
}

// Sanity: an empty header row should be replaced with synthetic A,B,... labels.
func TestRenderGetValues_BlankHeader(t *testing.T) {
	g := &gsheets{}
	body := []byte(`{"range":"S","majorDimension":"ROWS","values":[["",""],["x","y"]]}`)
	md, ok := g.RenderMarkdown(mcp.ToolName("gsheets_get_values"), body)
	require.True(t, ok)
	s := string(md)
	assert.True(t, strings.Contains(s, "| A | B |"), "expected synthetic A,B header; got %s", s)
}
