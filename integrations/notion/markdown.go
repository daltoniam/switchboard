package notion

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/markdown"
)

// ── Semantic domain types for the rendering layer ─────────────────────
// These types are the parse boundary: RenderMarkdown parses JSON into them,
// rendering functions accept only typed data. No map[string]any survives
// past the parse point.

// renderedPage is a Notion page parsed for markdown rendering.
type renderedPage struct {
	ID             string
	Title          string
	LastEditedTime int64
	Blocks         []renderedBlock
}

// renderedBlock is a single Notion block parsed for markdown rendering.
type renderedBlock struct {
	ID           string
	Type         string
	Text         string // rich text already converted to inline markdown
	CodeLanguage string
	IsChecked    bool
}

// renderedThread is a Notion discussion thread parsed for markdown rendering.
type renderedThread struct {
	Resolved bool
	Comments []renderedComment
}

// renderedComment is a single Notion comment parsed for markdown rendering.
type renderedComment struct {
	Author    string
	CreatedAt int64
	Text      string // rich text already converted to inline markdown
}

// ── Parse boundary (RenderMarkdown) ──────────────────────────────────

// markdownRenderers maps tool names to their markdown rendering functions
// for the legacy MarkdownIntegration path. Tools that opted into the views
// pipeline (get_page_content, retrieve_comments) bypass this map at runtime —
// processResult dispatches via the views renderer registry first. The map
// is left in place so RenderMarkdown stays callable for tools that have
// not yet migrated to views.
var markdownRenderers = map[mcp.ToolName]func([]byte) (markdown.Markdown, bool){}

// RenderMarkdown converts a tool's JSON response to Markdown.
// Implements mcp.MarkdownIntegration.
func (n *notion) RenderMarkdown(toolName mcp.ToolName, data []byte) (markdown.Markdown, bool) {
	if fn, ok := markdownRenderers[toolName]; ok {
		return fn(data)
	}
	return "", false
}

// renderPageContentMD parses the getPageContent JSON and renders as Markdown.
func renderPageContentMD(data []byte) (markdown.Markdown, bool) {
	var raw struct {
		Page   rawBlock   `json:"page"`
		Blocks []rawBlock `json:"blocks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	page := renderedPage{
		ID:             raw.Page.ID,
		Title:          richTextToMarkdown(raw.Page.Properties.Title),
		LastEditedTime: raw.Page.LastEditedTime,
		Blocks:         parseBlocks(raw.Blocks),
	}
	return pageToMarkdown(page), true
}

// renderCommentsMD parses the retrieveComments JSON and renders as Markdown.
func renderCommentsMD(data []byte) (markdown.Markdown, bool) {
	var raw struct {
		Results []rawThreadEntry `json:"results"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	threads := make([]renderedThread, len(raw.Results))
	for i, entry := range raw.Results {
		comments := make([]renderedComment, len(entry.Comments))
		for j, c := range entry.Comments {
			comments[j] = renderedComment{
				Author:    c.CreatedByID,
				CreatedAt: c.CreatedTime,
				Text:      richTextToMarkdown(c.Text),
			}
		}
		threads[i] = renderedThread{
			Resolved: entry.Discussion.Resolved,
			Comments: comments,
		}
	}
	return commentsToMarkdown(threads), true
}

// ── Raw JSON parse types (private, used only at the parse boundary) ──

// rawBlock maps the Notion v3 block JSON structure for deserialization.
type rawBlock struct {
	ID             string        `json:"id"`
	Type           string        `json:"type"`
	Properties     rawProperties `json:"properties"`
	Format         rawFormat     `json:"format"`
	LastEditedTime int64         `json:"last_edited_time"`
}

type rawProperties struct {
	Title   []any `json:"title"`
	Checked []any `json:"checked"`
}

type rawFormat struct {
	CodeLanguage string `json:"code_language"`
}

type rawThreadEntry struct {
	Discussion rawDiscussion `json:"discussion"`
	Comments   []rawComment  `json:"comments"`
}

type rawDiscussion struct {
	Resolved bool `json:"resolved"`
}

type rawComment struct {
	CreatedByID string `json:"created_by_id"`
	CreatedTime int64  `json:"created_time"`
	Text        []any  `json:"text"`
}

// parseBlocks converts raw JSON blocks into typed renderedBlocks.
// Rich text is converted to inline markdown at parse time.
func parseBlocks(blocks []rawBlock) []renderedBlock {
	result := make([]renderedBlock, len(blocks))
	for i, b := range blocks {
		result[i] = renderedBlock{
			ID:           b.ID,
			Type:         b.Type,
			Text:         richTextToMarkdown(b.Properties.Title),
			CodeLanguage: strings.ToLower(b.Format.CodeLanguage),
			IsChecked:    isRawChecked(b.Properties.Checked),
		}
	}
	return result
}

// isRawChecked interprets the Notion v3 checked property: [["Yes"]] → true.
func isRawChecked(checked []any) bool {
	if len(checked) == 0 {
		return false
	}
	first, _ := checked[0].([]any)
	if len(first) == 0 {
		return false
	}
	val, _ := first[0].(string)
	return val == "Yes"
}

// ── Rich text conversion (Notion v3 format → inline Markdown) ────────

// richTextToMarkdown converts a Notion v3 rich text array to inline Markdown.
//
// Notion v3 rich text is a double-nested array:
//
//	[["plain text"]]
//	[["bold", [["b"]]]]
//	[["linked", [["a", "https://url"]]]]
//	[["bold italic", [["b"], ["i"]]]]
//	[["segment 1"], ["segment 2", [["b"]]]]
func richTextToMarkdown(richText []any) string {
	if len(richText) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, segment := range richText {
		arr, ok := segment.([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		text, _ := arr[0].(string)
		if len(arr) < 2 {
			sb.WriteString(text)
			continue
		}

		formats, ok := arr[1].([]any)
		if !ok {
			sb.WriteString(text)
			continue
		}

		var hasBold, hasItalic, hasCode, hasStrike bool
		var linkURL string

		for _, f := range formats {
			fArr, ok := f.([]any)
			if !ok || len(fArr) == 0 {
				continue
			}
			code, _ := fArr[0].(string)
			switch code {
			case "b":
				hasBold = true
			case "i":
				hasItalic = true
			case "c":
				hasCode = true
			case "s":
				hasStrike = true
			case "a":
				if len(fArr) > 1 {
					linkURL, _ = fArr[1].(string)
				}
			}
		}

		markdown.ApplyMarks(&sb, text, hasBold, hasItalic, hasCode, hasStrike, linkURL)
	}

	return sb.String()
}

// ── Typed rendering functions (accept only semantic types) ───────────

// blockPrefix maps block types with simple "prefix + text + \n" rendering.
var blockPrefix = map[string]string{
	"header": "# ", "sub_header": "## ", "sub_sub_header": "### ",
	"bulleted_list": "- ", "quote": "> ", "callout": "> ",
}

// listTypes is the set of block types that form contiguous list groups.
var listTypes = map[string]bool{
	"bulleted_list": true, "numbered_list": true, "to_do": true,
}

func blocksToMarkdown(blocks []renderedBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	var sb strings.Builder
	numberedIdx := 0
	prevType := ""

	for _, block := range blocks {
		if prevType != "" && (!listTypes[prevType] || !listTypes[block.Type]) {
			sb.WriteString("\n")
		}

		if prefix, ok := blockPrefix[block.Type]; ok {
			writeBlockID(&sb, block.ID)
			sb.WriteString(prefix + block.Text + "\n")
			numberedIdx = 0
			prevType = block.Type
			continue
		}

		switch block.Type {
		case "text":
			writeBlockID(&sb, block.ID)
			if block.Text != "" {
				sb.WriteString(block.Text + "\n")
			} else {
				sb.WriteString("\n")
			}
			numberedIdx = 0
		case "numbered_list":
			numberedIdx++
			writeBlockID(&sb, block.ID)
			fmt.Fprintf(&sb, "%d. %s\n", numberedIdx, block.Text)
		case "to_do":
			writeBlockID(&sb, block.ID)
			if block.IsChecked {
				sb.WriteString("- [x] " + block.Text + "\n")
			} else {
				sb.WriteString("- [ ] " + block.Text + "\n")
			}
			numberedIdx = 0
		case "code":
			writeBlockID(&sb, block.ID)
			sb.WriteString("```" + block.CodeLanguage + "\n")
			sb.WriteString(block.Text + "\n")
			sb.WriteString("```\n")
			numberedIdx = 0
		case "divider":
			sb.WriteString("---\n")
			numberedIdx = 0
		case "toggle":
			writeBlockID(&sb, block.ID)
			sb.WriteString("**" + block.Text + "** (toggle)\n")
			numberedIdx = 0
		default:
			writeBlockID(&sb, block.ID)
			if block.Text != "" {
				sb.WriteString(block.Text + "\n")
			} else {
				sb.WriteString("\n")
			}
			numberedIdx = 0
		}

		prevType = block.Type
	}

	return sb.String()
}

func writeBlockID(sb *strings.Builder, id string) {
	if id != "" {
		fmt.Fprintf(sb, "<!-- block:%s -->\n", id)
	}
}

func pageToMarkdown(page renderedPage) markdown.Markdown {
	b := markdown.NewBuilder()
	b.Metadata("notion", "page_id", page.ID)
	b.Heading(1, page.Title)

	if page.LastEditedTime > 0 {
		b.Attribution("Last edited: " + millisToTimeString(page.LastEditedTime))
	}

	body := blocksToMarkdown(page.Blocks)
	if body != "" {
		b.BlankLine()
		b.Raw(body)
	}

	return b.Build()
}

func commentsToMarkdown(threads []renderedThread) markdown.Markdown {
	if len(threads) == 0 {
		return markdown.NoComments
	}

	b := markdown.NewBuilder()
	threadWord := "thread"
	if len(threads) != 1 {
		threadWord = "threads"
	}
	b.Heading(2, fmt.Sprintf("Comments (%d %s)", len(threads), threadWord))

	for i, thread := range threads {
		status := "open"
		if thread.Resolved {
			status = "resolved"
		}
		b.BlankLine()
		b.Heading(3, fmt.Sprintf("Thread %d (%s)", i+1, status))

		for _, c := range thread.Comments {
			ts := ""
			if c.CreatedAt > 0 {
				ts = millisToTimeString(c.CreatedAt)
			}
			b.BlockquoteAttribution(c.Author, ts, c.Text)
		}
	}

	return b.Build()
}

func millisToTimeString(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04 UTC")
}

// ── View renderer bridges ────────────────────────────────────────────
//
// Renderers registered via compactRenderers in notion.go. Each one
// accepts the post-CompactAny projection and produces markdown bytes.
// Errors from these bridges surface as view_dispatch_failed envelopes
// to the LLM, so the failure mode (an unexpected shape from CompactAny)
// is visible rather than silently degraded.

// renderSearchTitlesMD renders the search titles view as a markdown table.
// The titles view projects id, type, parent_id, collection_id,
// properties.title, url — the renderer surfaces title (linked to url),
// type, and id so LLMs can drill in by id.
func renderSearchTitlesMD(projected any) ([]byte, error) {
	root, ok := projected.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("notion: search titles renderer: expected object root, got %T", projected)
	}
	rawResults, _ := root["results"].([]any)

	b := markdown.NewBuilder()
	b.Metadata("notion", "tool", "search", "view", "titles")
	if len(rawResults) == 0 {
		b.Raw("No results.\n")
		return []byte(b.Build()), nil
	}

	rows := make([][]string, 0, len(rawResults)+1)
	rows = append(rows, []string{"Title", "Type", "ID"})
	for _, r := range rawResults {
		entry, ok := r.(map[string]any)
		if !ok {
			continue
		}
		title := searchResultTitle(entry)
		typ, _ := entry["type"].(string)
		id, _ := entry["id"].(string)
		url, _ := entry["url"].(string)
		linkedTitle := title
		if url != "" && title != "" {
			linkedTitle = fmt.Sprintf("[%s](%s)", title, url)
		}
		rows = append(rows, []string{linkedTitle, typ, id})
	}

	var sb strings.Builder
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())
	return []byte(b.Build()), nil
}

// renderQuerySummaryMD renders the query_data_source summary view as a
// markdown table. Surfaces id, title, last_edited so LLMs can find the
// target row before calling again with view=full for full properties.
func renderQuerySummaryMD(projected any) ([]byte, error) {
	root, ok := projected.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("notion: query summary renderer: expected object root, got %T", projected)
	}
	rawResults, _ := root["results"].([]any)

	b := markdown.NewBuilder()
	b.Metadata("notion", "tool", "query_data_source", "view", "summary")
	if len(rawResults) == 0 {
		b.Raw("No rows.\n")
		return []byte(b.Build()), nil
	}

	rows := make([][]string, 0, len(rawResults)+1)
	rows = append(rows, []string{"ID", "Title", "Last Edited"})
	for _, r := range rawResults {
		entry, ok := r.(map[string]any)
		if !ok {
			continue
		}
		id, _ := entry["id"].(string)
		title := titleFromProperties(entry["properties"])
		lastEdited := ""
		if ms := millisFromAny(entry["last_edited_time"]); ms > 0 {
			lastEdited = millisToTimeString(ms)
		}
		rows = append(rows, []string{id, title, lastEdited})
	}

	var sb strings.Builder
	markdown.WriteTable(&sb, rows)
	b.Raw(sb.String())
	return []byte(b.Build()), nil
}

// renderFullCommentsMD bridges the legacy raw-bytes renderCommentsMD into
// the typed Renderer signature used by the views renderer registry. Same
// shape as renderFullPageContentMD in notion.go.
func renderFullCommentsMD(projected any) ([]byte, error) {
	data, err := json.Marshal(projected)
	if err != nil {
		return nil, fmt.Errorf("notion: marshal projected comments: %w", err)
	}
	md, ok := renderCommentsMD(data)
	if !ok {
		return nil, fmt.Errorf("notion: renderCommentsMD declined to render (unexpected shape)")
	}
	return []byte(md), nil
}

// titleFromProperties pulls the title text out of a Notion v3 properties
// object. Returns "" when title is missing — the table cell stays blank
// rather than failing the whole render.
func titleFromProperties(props any) string {
	m, ok := props.(map[string]any)
	if !ok {
		return ""
	}
	rt, _ := m["title"].([]any)
	return richTextToMarkdown(rt)
}

// searchResultTitle resolves a displayable title for one search result.
// Notion v3 search populates either properties.title (block resolved in
// recordMap) or highlight.title/highlight.text (block unresolved). The
// renderer prefers the cleanest source; the empty case returns "" so
// the table cell stays blank rather than failing the whole render.
func searchResultTitle(entry map[string]any) string {
	if t := titleFromProperties(entry["properties"]); t != "" {
		return t
	}
	hl, ok := entry["highlight"].(map[string]any)
	if !ok {
		return ""
	}
	if t, _ := hl["title"].(string); t != "" {
		return t
	}
	t, _ := hl["text"].(string)
	return t
}

// millisFromAny extracts an integer milliseconds value from a JSON
// number (encoding/json decodes integers into float64 unless told
// otherwise). Returns 0 on any mismatch — the renderer will then omit
// the timestamp from the table.
func millisFromAny(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}
