package gsheets

import (
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Parse boundary ──────────────────────────────────────────────────

var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){
	"gsheets_get_values":       renderGetValuesMD,
	"gsheets_batch_get_values": renderBatchGetValuesMD,
	"gsheets_get_spreadsheet":  renderSpreadsheetMD,
}

func (g *gsheets) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// ── Raw types ───────────────────────────────────────────────────────

type rawValueRange struct {
	Range          string  `json:"range"`
	MajorDimension string  `json:"majorDimension"`
	Values         [][]any `json:"values"`
}

type rawBatchGetValues struct {
	SpreadsheetID string          `json:"spreadsheetId"`
	ValueRanges   []rawValueRange `json:"valueRanges"`
}

type rawSpreadsheet struct {
	SpreadsheetID  string          `json:"spreadsheetId"`
	SpreadsheetURL string          `json:"spreadsheetUrl"`
	Properties     rawSpreadsheetP `json:"properties"`
	Sheets         []rawSheet      `json:"sheets"`
}

type rawSpreadsheetP struct {
	Title    string `json:"title"`
	Locale   string `json:"locale"`
	TimeZone string `json:"timeZone"`
}

type rawSheet struct {
	Properties rawSheetProps `json:"properties"`
	Data       []rawGridData `json:"data"`
}

type rawSheetProps struct {
	SheetID        int64             `json:"sheetId"`
	Title          string            `json:"title"`
	Index          int               `json:"index"`
	SheetType      string            `json:"sheetType"`
	Hidden         bool              `json:"hidden"`
	GridProperties rawGridProperties `json:"gridProperties"`
}

type rawGridProperties struct {
	RowCount       int `json:"rowCount"`
	ColumnCount    int `json:"columnCount"`
	FrozenRowCount int `json:"frozenRowCount"`
	FrozenColCount int `json:"frozenColumnCount"`
}

type rawGridData struct {
	StartRow    int          `json:"startRow"`
	StartColumn int          `json:"startColumn"`
	RowData     []rawRowData `json:"rowData"`
}

type rawRowData struct {
	Values []rawCellData `json:"values"`
}

type rawCellData struct {
	FormattedValue string `json:"formattedValue"`
}

// ── Rendering ───────────────────────────────────────────────────────

func renderGetValuesMD(data []byte) (markdown.Markdown, bool) {
	var vr rawValueRange
	if err := json.Unmarshal(data, &vr); err != nil {
		return "", false
	}
	if vr.Range == "" {
		// Not a value-range response; bail out so the framework falls
		// back to raw JSON.
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gsheets", "range", vr.Range, "rows", fmt.Sprintf("%d", len(vr.Values)))
	b.Heading(2, "Range: "+vr.Range)
	b.BlankLine()
	writeValueTable(b, vr.Values, vr.MajorDimension)
	return b.Build(), true
}

func renderBatchGetValuesMD(data []byte) (markdown.Markdown, bool) {
	var resp rawBatchGetValues
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", false
	}
	if resp.SpreadsheetID == "" && len(resp.ValueRanges) == 0 {
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gsheets", "spreadsheet_id", resp.SpreadsheetID, "ranges", fmt.Sprintf("%d", len(resp.ValueRanges)))
	b.Heading(1, "Batch values")
	b.BlankLine()
	for _, vr := range resp.ValueRanges {
		b.Heading(2, "Range: "+vr.Range)
		b.BlankLine()
		writeValueTable(b, vr.Values, vr.MajorDimension)
		b.BlankLine()
	}
	return b.Build(), true
}

func renderSpreadsheetMD(data []byte) (markdown.Markdown, bool) {
	var s rawSpreadsheet
	if err := json.Unmarshal(data, &s); err != nil {
		return "", false
	}
	if s.SpreadsheetID == "" {
		// Not a spreadsheet resource (e.g. an error envelope) — bail
		// out so the framework falls back to JSON.
		return "", false
	}

	b := markdown.NewBuilder()
	b.Metadata("gsheets", "spreadsheet_id", s.SpreadsheetID)

	title := s.Properties.Title
	if title == "" {
		title = "(untitled spreadsheet)"
	}
	b.Heading(1, title)
	if s.Properties.Locale != "" || s.Properties.TimeZone != "" {
		b.Attribution("Locale: "+s.Properties.Locale, "Time zone: "+s.Properties.TimeZone)
	}
	if s.SpreadsheetURL != "" {
		b.Raw("[Open in Google Sheets](" + s.SpreadsheetURL + ")\n")
	}
	b.BlankLine()

	b.Heading(2, fmt.Sprintf("Sheets (%d)", len(s.Sheets)))
	b.BlankLine()
	for _, sheet := range s.Sheets {
		renderSheetSection(b, &sheet)
	}
	return b.Build(), true
}

func renderSheetSection(b *markdown.Builder, sheet *rawSheet) {
	title := sheet.Properties.Title
	if title == "" {
		title = fmt.Sprintf("(unnamed sheet %d)", sheet.Properties.SheetID)
	}
	b.Heading(3, title)
	attrs := []string{
		fmt.Sprintf("sheet_id=%d", sheet.Properties.SheetID),
		fmt.Sprintf("%dr × %dc", sheet.Properties.GridProperties.RowCount, sheet.Properties.GridProperties.ColumnCount),
	}
	if sheet.Properties.Hidden {
		attrs = append(attrs, "hidden")
	}
	if sheet.Properties.SheetType != "" && sheet.Properties.SheetType != "GRID" {
		attrs = append(attrs, "type="+sheet.Properties.SheetType)
	}
	b.Attribution(attrs...)

	if len(sheet.Data) == 0 {
		b.BlankLine()
		return
	}
	// When grid data is included, emit each grid block as a table.
	for _, grid := range sheet.Data {
		if len(grid.RowData) == 0 {
			continue
		}
		rows := make([][]any, 0, len(grid.RowData))
		for _, rd := range grid.RowData {
			row := make([]any, 0, len(rd.Values))
			for _, cell := range rd.Values {
				row = append(row, cell.FormattedValue)
			}
			rows = append(rows, row)
		}
		b.BlankLine()
		writeValueTable(b, rows, "ROWS")
		b.BlankLine()
	}
}

// writeValueTable emits a 2-D slice as a Markdown table. The first row is
// treated as a header. Empty inputs render as a friendly "no values" note.
// When majorDimension is COLUMNS we transpose so the rendered table is
// still readable in row-major order.
func writeValueTable(b *markdown.Builder, values [][]any, majorDimension string) {
	if len(values) == 0 {
		b.Raw("_(no values)_\n")
		return
	}
	if majorDimension == "COLUMNS" {
		values = transpose(values)
	}
	cols := 0
	for _, row := range values {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		b.Raw("_(no values)_\n")
		return
	}

	rows := make([][]string, 0, len(values))
	for _, row := range values {
		rendered := make([]string, cols)
		for i := 0; i < cols; i++ {
			if i < len(row) {
				rendered[i] = pipeSafe(cellString(row[i]))
			}
		}
		rows = append(rows, rendered)
	}

	// If the first row looks empty, synthesize a column header so the
	// table renders as Markdown (Markdown requires a non-empty header).
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

// cellString renders a Sheets API cell value (any of string / number /
// bool / null) as a printable string. Numbers are rendered without their
// trailing .0 when integral, matching what a human would expect to see.
func cellString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case map[string]any, []any:
		// Cell objects (e.g. ErrorValue) — fall back to JSON repr.
		buf, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(buf)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func pipeSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

func transpose(values [][]any) [][]any {
	cols := 0
	for _, col := range values {
		if len(col) > cols {
			cols = len(col)
		}
	}
	out := make([][]any, cols)
	for r := 0; r < cols; r++ {
		row := make([]any, len(values))
		for c, col := range values {
			if r < len(col) {
				row[c] = col[r]
			}
		}
		out[r] = row
	}
	return out
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
	// reverse
	b := sb.String()
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return string(out)
}
