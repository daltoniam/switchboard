package gslides

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

// ── Presentation lifecycle ──────────────────────────────────────────

func getPresentation(ctx context.Context, g *gslides, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	presentationID := r.Str("presentation_id")
	fields := r.Str("fields")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if presentationID == "" {
		return mcp.ErrResult(fmt.Errorf("gslides_get_presentation: presentation_id is required"))
	}

	path := "/presentations/" + url.PathEscape(presentationID)
	if fields != "" {
		params := url.Values{}
		params.Set("fields", fields)
		path += "?" + params.Encode()
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPresentation(ctx context.Context, g *gslides, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{}
	if title != "" {
		body["title"] = title
	}

	data, err := g.post(ctx, "/presentations", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Page (slide) access ─────────────────────────────────────────────

func getPage(ctx context.Context, g *gslides, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	presentationID := r.Str("presentation_id")
	pageObjectID := r.Str("page_object_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if presentationID == "" {
		return mcp.ErrResult(fmt.Errorf("gslides_get_page: presentation_id is required"))
	}
	if pageObjectID == "" {
		return mcp.ErrResult(fmt.Errorf("gslides_get_page: page_object_id is required"))
	}

	path := "/presentations/" + url.PathEscape(presentationID) + "/pages/" + url.PathEscape(pageObjectID)
	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPageThumbnail(ctx context.Context, g *gslides, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	presentationID := r.Str("presentation_id")
	pageObjectID := r.Str("page_object_id")
	thumbnailSize := r.Str("thumbnail_size")
	mimeType := r.Str("mime_type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if presentationID == "" {
		return mcp.ErrResult(fmt.Errorf("gslides_get_page_thumbnail: presentation_id is required"))
	}
	if pageObjectID == "" {
		return mcp.ErrResult(fmt.Errorf("gslides_get_page_thumbnail: page_object_id is required"))
	}

	params := url.Values{}
	if thumbnailSize != "" {
		params.Set("thumbnailProperties.thumbnailSize", thumbnailSize)
	}
	if mimeType != "" {
		params.Set("thumbnailProperties.mimeType", mimeType)
	}

	path := "/presentations/" + url.PathEscape(presentationID) + "/pages/" + url.PathEscape(pageObjectID) + "/thumbnail"
	if enc := params.Encode(); enc != "" {
		path += "?" + enc
	}

	data, err := g.get(ctx, "%s", path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// ── Presentation-level batchUpdate ──────────────────────────────────

// batchUpdate accepts a JSON-encoded `requests` array and forwards it to
// presentations.batchUpdate. Same shape as the gdocs/gsheets batch_update tools.
func batchUpdate(ctx context.Context, g *gslides, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	presentationID := r.Str("presentation_id")
	requestsRaw := r.Str("requests")
	writeControlRevision := r.Str("write_control_revision")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if presentationID == "" {
		return mcp.ErrResult(fmt.Errorf("gslides_batch_update: presentation_id is required"))
	}
	if requestsRaw == "" {
		return mcp.ErrResult(fmt.Errorf("gslides_batch_update: requests is required"))
	}

	var requests []any
	if err := json.Unmarshal([]byte(requestsRaw), &requests); err != nil {
		return mcp.ErrResult(fmt.Errorf("gslides_batch_update: requests must be a JSON array: %w", err))
	}

	body := map[string]any{"requests": requests}
	if writeControlRevision != "" {
		body["writeControl"] = map[string]any{"requiredRevisionId": writeControlRevision}
	}

	data, err := g.post(ctx, "/presentations/"+url.PathEscape(presentationID)+":batchUpdate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
