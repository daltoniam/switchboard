package gdocs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// ── Document lifecycle ──────────────────────────────────────────────

func getDocument(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("document_id")
	suggestionsView := r.Str("suggestions_view_mode")
	includeTabs := r.Str("include_tabs_content")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_get_document: document_id is required"))
	}

	params := url.Values{}
	if suggestionsView != "" {
		params.Set("suggestionsViewMode", suggestionsView)
	}
	if includeTabs != "" {
		params.Set("includeTabsContent", includeTabs)
	}

	path := "/documents/" + url.PathEscape(docID)
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDocument(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{}
	if title != "" {
		body["title"] = title
	}

	data, err := g.post(ctx, "/documents", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Edits ───────────────────────────────────────────────────────────

// batchUpdate accepts a JSON-encoded `requests` array and forwards it to
// documents.batchUpdate. The requests payload is large and shaped exactly
// like the Docs API expects, so we don't try to second-guess it — we just
// require that it parses as an array.
func batchUpdate(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("document_id")
	requestsRaw := r.Str("requests")
	requiredRev := r.Str("write_control_revision")
	targetRev := r.Str("write_control_target")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_batch_update: document_id is required"))
	}
	if requestsRaw == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_batch_update: requests is required"))
	}

	var requests []any
	if err := json.Unmarshal([]byte(requestsRaw), &requests); err != nil {
		return mcp.ErrResult(fmt.Errorf("gdocs_batch_update: requests must be a JSON array: %w", err))
	}

	body := map[string]any{"requests": requests}
	if requiredRev != "" || targetRev != "" {
		wc := map[string]any{}
		if requiredRev != "" {
			wc["requiredRevisionId"] = requiredRev
		}
		if targetRev != "" {
			wc["targetRevisionId"] = targetRev
		}
		body["writeControl"] = wc
	}

	data, err := g.post(ctx, "/documents/"+url.PathEscape(docID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// insertText is a convenience wrapper that builds a single insertText
// request and forwards it through documents.batchUpdate.
func insertText(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("document_id")
	text := r.Str("text")
	index := r.Int("index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_insert_text: document_id is required"))
	}
	if text == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_insert_text: text is required"))
	}
	if index < 1 {
		return mcp.ErrResult(fmt.Errorf("gdocs_insert_text: index must be >= 1 (index 0 is reserved by the Docs API)"))
	}

	req := map[string]any{
		"insertText": map[string]any{
			"location": map[string]any{"index": index},
			"text":     text,
		},
	}
	body := map[string]any{"requests": []any{req}}
	data, err := g.post(ctx, "/documents/"+url.PathEscape(docID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// appendText fetches the document, computes the index just before the
// trailing newline (Docs API requires inserts at endIndex-1), and inserts.
// Optionally prepends a newline for clean separation from existing content.
func appendText(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("document_id")
	text := r.Str("text")
	leadingNewlineStr := r.Str("leading_newline")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_append_text: document_id is required"))
	}
	if text == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_append_text: text is required"))
	}
	leadingNewline := leadingNewlineStr != "false"

	// Fetch the doc to find the end index of the body.
	docData, err := g.get(ctx, "/documents/%s?fields=body(content(endIndex))", url.PathEscape(docID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	endIndex, err := lastBodyEndIndex(docData)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("gdocs_append_text: %w", err))
	}
	// The Docs API places a trailing newline at endIndex-1; inserts at
	// endIndex itself fail. Subtract 1 to land before the newline.
	insertAt := endIndex - 1
	if insertAt < 1 {
		insertAt = 1
	}

	payload := text
	if leadingNewline {
		payload = "\n" + text
	}

	req := map[string]any{
		"insertText": map[string]any{
			"location": map[string]any{"index": insertAt},
			"text":     payload,
		},
	}
	body := map[string]any{"requests": []any{req}}
	data, err := g.post(ctx, "/documents/"+url.PathEscape(docID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func replaceText(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("document_id")
	find := r.Str("find")
	replace := r.Str("replace")
	matchCaseStr := r.Str("match_case")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_replace_text: document_id is required"))
	}
	if find == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_replace_text: find is required (cannot search for empty string)"))
	}
	matchCase := matchCaseStr == "true"

	req := map[string]any{
		"replaceAllText": map[string]any{
			"containsText": map[string]any{
				"text":      find,
				"matchCase": matchCase,
			},
			"replaceText": replace,
		},
	}
	body := map[string]any{"requests": []any{req}}
	data, err := g.post(ctx, "/documents/"+url.PathEscape(docID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteContent(ctx context.Context, g *gdocs, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docID := r.Str("document_id")
	startIndex := r.Int("start_index")
	endIndex := r.Int("end_index")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if docID == "" {
		return mcp.ErrResult(fmt.Errorf("gdocs_delete_content: document_id is required"))
	}
	if startIndex < 1 {
		return mcp.ErrResult(fmt.Errorf("gdocs_delete_content: start_index must be >= 1"))
	}
	if endIndex <= startIndex {
		return mcp.ErrResult(fmt.Errorf("gdocs_delete_content: end_index (%d) must be > start_index (%d)", endIndex, startIndex))
	}

	req := map[string]any{
		"deleteContentRange": map[string]any{
			"range": map[string]any{
				"startIndex": startIndex,
				"endIndex":   endIndex,
			},
		},
	}
	body := map[string]any{"requests": []any{req}}
	data, err := g.post(ctx, "/documents/"+url.PathEscape(docID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// lastBodyEndIndex walks the body.content array and returns the highest
// endIndex it can find. The Docs API returns content as a flat array of
// StructuralElements, each carrying its own startIndex/endIndex; the last
// element's endIndex is the document end.
func lastBodyEndIndex(data []byte) (int, error) {
	var resp struct {
		Body struct {
			Content []struct {
				EndIndex int `json:"endIndex"`
			} `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("failed to parse document body: %w", err)
	}
	if len(resp.Body.Content) == 0 {
		return 0, fmt.Errorf("document body has no content")
	}
	max := 0
	for _, el := range resp.Body.Content {
		if el.EndIndex > max {
			max = el.EndIndex
		}
	}
	if max == 0 {
		return 0, fmt.Errorf("document body content has no endIndex")
	}
	return max, nil
}
