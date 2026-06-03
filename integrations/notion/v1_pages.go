package notion

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Pages (v1) ---
//
// Notion v1 page semantics:
//   - parent.page_id      → subpage under a page
//   - parent.database_id  → row in a database (v1 alias for data_source_id)
//   - parent.data_source_id (2025-09-03) → row in a specific data source
//   - parent.workspace    → workspace-level page (public OAuth apps only)
//
// The v3 backend used a single "parent" arg with page_id OR database_id.
// The v1 backend accepts the same shape and translates: database_id is
// passed through as-is so callers don't have to relearn the schema, but
// callers using the newer model may also pass data_source_id directly.

func v1CreatePage(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parent := r.Map("parent")
	properties := r.Map("properties")
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if parent == nil {
		return mcp.ErrResult(fmt.Errorf("parent is required"))
	}

	body, err := v1BuildPageBody(parent, properties, title, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}

	data, err := n.post(ctx, "/pages", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1RetrievePage(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	pageID, err := mcp.ArgStr(args, "page_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}
	data, err := n.get(ctx, "/pages/%s", pageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1UpdatePage(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	props := r.Map("properties")
	archived := r.Bool("archived")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}

	body := map[string]any{}
	if props != nil {
		body["properties"] = props
	}
	if archived {
		// Notion accepts both keys; "archived" is the older field name
		// (2022-06-28) and "in_trash" is the 2025-09-03 spelling. Send
		// "archived" for broadest compatibility — both versions of the
		// API accept it.
		body["archived"] = true
	}
	if len(body) == 0 {
		return mcp.ErrResult(fmt.Errorf("at least one of properties or archived must be set"))
	}

	data, err := n.patch(ctx, "/pages/"+pageID, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func v1MovePage(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	newParent := r.Map("parent")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}
	if newParent == nil {
		return mcp.ErrResult(fmt.Errorf("parent is required"))
	}

	parentObj, err := v1BuildParentObject(newParent)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := n.patch(ctx, "/pages/"+pageID, map[string]any{"parent": parentObj})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// v1RetrievePageProperty hits the dedicated property-retrieval endpoint
// rather than fetching the whole page. This matches the v3 handler's
// contract (returns just the requested property value) but is more
// efficient for paginated relations/rollups that exceed 25 references.
func v1RetrievePageProperty(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	propertyID := r.Str("property_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}
	if propertyID == "" {
		return mcp.ErrResult(fmt.Errorf("property_id is required"))
	}
	data, err := n.get(ctx, "/pages/%s/properties/%s", pageID, propertyID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// v1GetPageContent fetches a page plus its child blocks in one logical
// call. Internally it issues GET /pages/{id} + GET /blocks/{id}/children,
// paginating up to `limit` blocks (default 100, capped at one page of
// block children to avoid runaway requests).
func v1GetPageContent(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	pageID := r.Str("page_id")
	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if pageID == "" {
		return mcp.ErrResult(fmt.Errorf("page_id is required"))
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100 // single page; callers needing more should paginate get_block_children
	}

	pageRaw, err := n.get(ctx, "/pages/%s", pageID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var page map[string]any
	if err := json.Unmarshal(pageRaw, &page); err != nil {
		return mcp.ErrResult(fmt.Errorf("parse page: %w", err))
	}

	childrenRaw, err := n.get(ctx, "/blocks/%s/children?page_size=%d", pageID, limit)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var childrenResp struct {
		Results    []map[string]any `json:"results"`
		HasMore    bool             `json:"has_more"`
		NextCursor string           `json:"next_cursor"`
	}
	if err := json.Unmarshal(childrenRaw, &childrenResp); err != nil {
		return mcp.ErrResult(fmt.Errorf("parse block children: %w", err))
	}

	return mcp.JSONResult(map[string]any{
		"page":        page,
		"blocks":      childrenResp.Results,
		"has_more":    childrenResp.HasMore,
		"next_cursor": childrenResp.NextCursor,
	})
}

func v1CreatePageWithContent(ctx context.Context, n *notionV1, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	parent := r.Map("parent")
	properties := r.Map("properties")
	title := r.Str("title")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if parent == nil {
		return mcp.ErrResult(fmt.Errorf("parent is required"))
	}

	childrenAny, ok := args["children"].([]any)
	if !ok || len(childrenAny) == 0 {
		return mcp.ErrResult(fmt.Errorf("children is required"))
	}
	children := make([]map[string]any, 0, len(childrenAny))
	for _, c := range childrenAny {
		if cm, ok := c.(map[string]any); ok {
			children = append(children, v1NormalizeBlock(cm))
		}
	}

	body, err := v1BuildPageBody(parent, properties, title, children)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(children) > 100 {
		// /v1/pages caps children at 100; rather than silently truncate,
		// surface so the caller can split.
		return mcp.ErrResult(fmt.Errorf("v1 create_page_with_content accepts up to 100 children; got %d. Create the page first, then call append_block_children for the remainder", len(children)))
	}

	data, err := n.post(ctx, "/pages", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- helpers ---

// v1BuildPageBody builds the POST /pages request body shared by
// create_page and create_page_with_content. Children may be nil.
func v1BuildPageBody(parent, properties map[string]any, title string, children []map[string]any) (map[string]any, error) {
	parentObj, err := v1BuildParentObject(parent)
	if err != nil {
		return nil, err
	}

	body := map[string]any{"parent": parentObj}

	props := map[string]any{}
	for k, v := range properties {
		props[k] = v
	}
	if title != "" {
		// Notion's title property is conventionally keyed "title". For
		// rows in a database the actual title column may be user-named
		// (e.g. "Name"), in which case the caller is expected to supply
		// the property explicitly via the properties map — we only
		// auto-fill when nothing collides with "title".
		const titleKey = "title"
		if _, exists := props[titleKey]; !exists {
			props[titleKey] = v1TitleRichText(title)
		}
	}
	if len(props) > 0 {
		body["properties"] = props
	}

	if len(children) > 0 {
		anyChildren := make([]any, len(children))
		for i, c := range children {
			anyChildren[i] = c
		}
		body["children"] = anyChildren
	}

	return body, nil
}

// v1BuildParentObject converts the caller-facing parent map into the
// shape Notion v1 expects. Inputs accepted:
//
//	{"page_id": "..."}        → {"type": "page_id", "page_id": "..."}
//	{"database_id": "..."}    → {"type": "database_id", "database_id": "..."}
//	{"data_source_id": "..."} → {"type": "data_source_id", "data_source_id": "..."}
//	{"workspace": true}       → {"type": "workspace", "workspace": true}
//
// If the caller already provided a "type" field, pass through.
func v1BuildParentObject(parent map[string]any) (map[string]any, error) {
	if t, ok := parent["type"].(string); ok && t != "" {
		return parent, nil
	}
	if v, ok := parent["page_id"].(string); ok && v != "" {
		return map[string]any{"type": "page_id", "page_id": v}, nil
	}
	if v, ok := parent["data_source_id"].(string); ok && v != "" {
		return map[string]any{"type": "data_source_id", "data_source_id": v}, nil
	}
	if v, ok := parent["database_id"].(string); ok && v != "" {
		return map[string]any{"type": "database_id", "database_id": v}, nil
	}
	if v, ok := parent["workspace"].(bool); ok && v {
		return map[string]any{"type": "workspace", "workspace": true}, nil
	}
	return nil, fmt.Errorf("parent must contain page_id, database_id, data_source_id, or workspace:true")
}

// v1TitleRichText builds the standard {title: [{type:text, text:{content:s}}]}
// shape used for the title property.
func v1TitleRichText(s string) map[string]any {
	return map[string]any{
		"title": []any{
			map[string]any{
				"type": "text",
				"text": map[string]any{"content": s},
			},
		},
	}
}

// v1NormalizeBlock translates the legacy v3-shaped block descriptors
// that current tool docs advertise:
//
//	{"type": "text", "properties": {"title": [["hello"]]}}
//
// into v1-shaped blocks:
//
//	{"object":"block","type":"paragraph","paragraph":{"rich_text":[{"type":"text","text":{"content":"hello"}}]}}
//
// Pure v1 blocks (those already containing `object`, or a top-level key
// matching their type like "paragraph", "heading_1", etc.) pass through
// unchanged so power-users can hand-roll v1 block JSON.
func v1NormalizeBlock(b map[string]any) map[string]any {
	if _, ok := b["object"]; ok {
		return b
	}
	blockType, _ := b["type"].(string)
	if blockType == "" {
		return b
	}
	// If the v1 type-keyed payload is already present, pass through.
	if _, ok := b[blockType]; ok {
		out := map[string]any{"object": "block"}
		for k, v := range b {
			out[k] = v
		}
		return out
	}
	// Map legacy v3 shape → v1.
	v3Type := blockType
	v1Type := v1BlockTypeMap[v3Type]
	if v1Type == "" {
		v1Type = "paragraph"
	}
	text := v1ExtractV3TitleText(b)
	payload := map[string]any{"rich_text": v1RichTextFromString(text)}

	// Code blocks need a language field.
	if v1Type == "code" {
		lang := "plain text"
		if format, ok := b["format"].(map[string]any); ok {
			if l, ok := format["code_language"].(string); ok && l != "" {
				lang = l
			}
		}
		payload["language"] = lang
	}
	// to_do needs a checked field.
	if v1Type == "to_do" {
		payload["checked"] = v1ExtractV3Checked(b)
	}

	return map[string]any{
		"object": "block",
		"type":   v1Type,
		v1Type:   payload,
	}
}

// v1BlockTypeMap maps the v3 block-type names that current tool docs
// advertise to v1 block-type names. Unmapped types fall back to
// "paragraph" (a safe text block).
var v1BlockTypeMap = map[string]string{
	"text":           "paragraph",
	"header":         "heading_1",
	"sub_header":     "heading_2",
	"sub_sub_header": "heading_3",
	"bulleted_list":  "bulleted_list_item",
	"numbered_list":  "numbered_list_item",
	"to_do":          "to_do",
	"quote":          "quote",
	"callout":        "callout",
	"code":           "code",
	"divider":        "divider",
	"toggle":         "toggle",
}

// v1ExtractV3TitleText pulls the plain text out of a v3 properties.title
// structure: [[ "hello", [["b"]] ], [ " world" ]]. Returns the concatenated
// plain text.
func v1ExtractV3TitleText(b map[string]any) string {
	props, ok := b["properties"].(map[string]any)
	if !ok {
		return ""
	}
	title, ok := props["title"].([]any)
	if !ok {
		return ""
	}
	var out string
	for _, run := range title {
		runArr, ok := run.([]any)
		if !ok || len(runArr) == 0 {
			continue
		}
		s, _ := runArr[0].(string)
		out += s
	}
	return out
}

// v1ExtractV3Checked reads the legacy v3 to_do "checked" flag. v3
// encodes it as properties.checked = [["Yes"]] (or absent for unchecked);
// anything else (or missing) is false.
func v1ExtractV3Checked(b map[string]any) bool {
	props, ok := b["properties"].(map[string]any)
	if !ok {
		return false
	}
	checked, ok := props["checked"].([]any)
	if !ok || len(checked) == 0 {
		return false
	}
	pair, ok := checked[0].([]any)
	if !ok || len(pair) == 0 {
		return false
	}
	v, ok := pair[0].(string)
	if !ok {
		return false
	}
	return v == "Yes" || v == "true"
}

// v1RichTextFromString wraps a plain string in the v1 rich_text shape.
func v1RichTextFromString(s string) []any {
	return []any{
		map[string]any{
			"type": "text",
			"text": map[string]any{"content": s},
		},
	}
}
