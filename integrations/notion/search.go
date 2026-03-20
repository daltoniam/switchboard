package notion

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// highlightTagRe matches Notion's proprietary highlight markup tags.
// These are random alphanumeric strings (e.g., <gzkNfoUU>), not standard HTML.
// The regex is intentionally broad (any alphanum tag) because Notion's tag format
// is undocumented and may vary. Blast radius is limited: only applied to the
// "highlight" field from search results, not arbitrary page content.
var highlightTagRe = regexp.MustCompile(`</?[a-zA-Z0-9]+>`)

// stripHighlightTags removes Notion's proprietary highlight markup
// (e.g., <gzkNfoUU>matched text</gzkNfoUU>) from search result strings.
func stripHighlightTags(s string) string {
	return highlightTagRe.ReplaceAllString(s, "")
}

func searchNotion(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	typeFilter := r.Str("type")
	limit := r.Int("limit")
	filters := r.Map("filters")
	sortArg := r.Map("sort")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"type":    "BlocksInSpace",
		"query":   query,
		"spaceId": n.spaceID,
		"limit":   20,
		"source":  "quick_find",
		"filters": map[string]any{
			"isDeletedOnly":                           false,
			"navigableBlockContentOnly":               false,
			"excludeTemplates":                        false,
			"requireEditPermissions":                  false,
			"includePublicPagesWithoutExplicitAccess": false,
			"ancestors":                               []string{},
			"createdBy":                               []string{},
			"editedBy":                                []string{},
			"lastEditedTime":                          map[string]any{},
			"createdTime":                             map[string]any{},
			"inTeams":                                 []string{},
			"excludeSurrogateCollections":             false,
			"excludedParentCollectionIds":             []string{},
		},
		"sort": map[string]any{"field": "relevance"},
	}

	if limit > 0 {
		if limit > 100 {
			limit = 100
		}
		body["limit"] = limit
	}
	if filters != nil {
		body["filters"] = filters
	}
	if sortArg != nil {
		body["sort"] = sortArg
	}

	data, err := n.doRequest(ctx, "/api/v3/search", body)
	if err != nil {
		return mcp.ErrResult(err)
	}

	// Normalize: merge results (id, highlight) with recordMap block data
	// into a flat {results: [{id, type, properties, highlight, ...}], total}
	var resp struct {
		Results []struct {
			ID        string `json:"id"`
			Highlight any    `json:"highlight"`
		} `json:"results"`
		Total     int                        `json:"total"`
		RecordMap map[string]json.RawMessage `json:"recordMap"`
	}
	if err := unmarshalJSON(data, &resp); err != nil {
		return mcp.ErrResult(err)
	}

	// Build block lookup from recordMap
	blocks := parseBlockTable(resp.RecordMap)

	// Merge results with block data — include all fields, let compaction strip noise.
	var results []map[string]any
	for _, sr := range resp.Results {
		entry := map[string]any{
			"id":        sr.ID,
			"highlight": cleanHighlight(sr.Highlight),
			"url":       "https://www.notion.so/" + strings.ReplaceAll(sr.ID, "-", ""),
		}
		block, ok := blocks[sr.ID]
		if !ok {
			results = append(results, entry)
			continue
		}
		for k, v := range block {
			if k == "id" {
				continue // already set from results array
			}
			entry[k] = v
		}
		results = append(results, entry)
	}

	// v3 has no server-side type filter — filter post-hoc by block type
	if typeFilter != "" {
		filtered := results[:0]
		for _, sr := range results {
			if matchesTypeFilter(sr, typeFilter) {
				filtered = append(filtered, sr)
			}
		}
		results = filtered
	}

	return mcp.JSONResult(map[string]any{
		"results": results,
		"total":   resp.Total,
	})
}

// cleanHighlight strips Notion's proprietary highlight markup tags from all
// string values in the highlight object (text, pathText, etc.).
func cleanHighlight(h any) any {
	m, ok := h.(map[string]any)
	if !ok {
		return h
	}
	cleaned := make(map[string]any, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			cleaned[k] = stripHighlightTags(s)
		} else {
			cleaned[k] = v
		}
	}
	return cleaned
}

// parseBlockTable extracts block records from a recordMap's "block" table.
// Returns an empty map if the table is missing or malformed.
func parseBlockTable(recordMap map[string]json.RawMessage) map[string]map[string]any {
	blocks := map[string]map[string]any{}
	rawBlocks, ok := recordMap["block"]
	if !ok {
		return blocks
	}
	var blockTable map[string]struct {
		Value map[string]any `json:"value"`
	}
	if err := unmarshalJSON(rawBlocks, &blockTable); err != nil {
		return blocks
	}
	for id, entry := range blockTable {
		if entry.Value != nil {
			blocks[id] = entry.Value
		}
	}
	return blocks
}

// matchesTypeFilter checks if a search result matches the user-requested type.
// Maps user-facing types to v3 block types:
//
//	"page"        -> block type "page"
//	"data_source" -> block type "collection_view_page" or "collection_view"
func matchesTypeFilter(result map[string]any, filter string) bool {
	blockType, _ := result["type"].(string)
	switch filter {
	case "page":
		return blockType == "page"
	case "data_source":
		return blockType == "collection_view_page" || blockType == "collection_view"
	default:
		return false
	}
}
