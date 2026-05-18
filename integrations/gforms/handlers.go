package gforms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

// ── Form lifecycle ──────────────────────────────────────────────────

func getForm(ctx context.Context, g *gforms, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	formID := r.Str("form_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if formID == "" {
		return mcp.ErrResult(fmt.Errorf("gforms_get_form: form_id is required"))
	}

	data, err := g.get(ctx, "/forms/%s", url.PathEscape(formID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createForm(ctx context.Context, g *gforms, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	documentTitle := r.Str("document_title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if title == "" {
		return mcp.ErrResult(fmt.Errorf("gforms_create_form: title is required"))
	}

	info := map[string]any{"title": title}
	if documentTitle != "" {
		info["documentTitle"] = documentTitle
	}
	body := map[string]any{"info": info}

	data, err := g.post(ctx, "/forms", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Form structure mutation ─────────────────────────────────────────

// batchUpdate accepts a JSON-encoded `requests` array and forwards it to
// forms.batchUpdate. Same shape as the gdocs/gsheets/gslides batch_update
// tools.
func batchUpdate(ctx context.Context, g *gforms, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	formID := r.Str("form_id")
	requestsRaw := r.Str("requests")
	includeFormInResponse := r.Bool("include_form_in_response")
	writeControlRevision := r.Str("write_control_revision")
	writeControlTargetRevision := r.Str("write_control_target_revision")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if formID == "" {
		return mcp.ErrResult(fmt.Errorf("gforms_batch_update: form_id is required"))
	}
	if requestsRaw == "" {
		return mcp.ErrResult(fmt.Errorf("gforms_batch_update: requests is required"))
	}

	var requests []any
	if err := json.Unmarshal([]byte(requestsRaw), &requests); err != nil {
		return mcp.ErrResult(fmt.Errorf("gforms_batch_update: requests must be a JSON array: %w", err))
	}

	body := map[string]any{"requests": requests}
	if includeFormInResponse {
		body["includeFormInResponse"] = true
	}
	wc := map[string]any{}
	if writeControlRevision != "" {
		wc["requiredRevisionId"] = writeControlRevision
	}
	if writeControlTargetRevision != "" {
		wc["targetRevisionId"] = writeControlTargetRevision
	}
	if len(wc) > 0 {
		body["writeControl"] = wc
	}

	data, err := g.post(ctx, "/forms/"+url.PathEscape(formID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Response access ─────────────────────────────────────────────────

func listResponses(ctx context.Context, g *gforms, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	formID := r.Str("form_id")
	pageSize := r.OptInt("page_size", 0)
	pageToken := r.Str("page_token")
	filter := r.Str("filter")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if formID == "" {
		return mcp.ErrResult(fmt.Errorf("gforms_list_responses: form_id is required"))
	}

	params := url.Values{}
	if pageSize > 0 {
		params.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	if filter != "" {
		params.Set("filter", filter)
	}

	path := "/forms/" + url.PathEscape(formID) + "/responses"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getResponse(ctx context.Context, g *gforms, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	formID := r.Str("form_id")
	responseID := r.Str("response_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if formID == "" {
		return mcp.ErrResult(fmt.Errorf("gforms_get_response: form_id is required"))
	}
	if responseID == "" {
		return mcp.ErrResult(fmt.Errorf("gforms_get_response: response_id is required"))
	}

	path := "/forms/" + url.PathEscape(formID) + "/responses/" + url.PathEscape(responseID)
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
