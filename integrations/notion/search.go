package notion

import (
	"context"
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
)

func searchNotion(ctx context.Context, n *notion, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"type":    "BlocksInSpace",
		"query":   argStr(args, "query"),
		"spaceId": n.spaceID,
		"limit":   20,
		"source":  "quick_find",
		"filters": map[string]any{
			"isDeletedOnly":                            false,
			"navigableBlockContentOnly":                false,
			"excludeTemplates":                         false,
			"requireEditPermissions":                   false,
			"includePublicPagesWithoutExplicitAccess":  false,
			"ancestors":                                []string{},
			"createdBy":                                []string{},
			"editedBy":                                 []string{},
			"lastEditedTime":                           map[string]any{},
			"createdTime":                              map[string]any{},
			"inTeams":                                  []string{},
			"excludeSurrogateCollections":              false,
			"excludedParentCollectionIds":              []string{},
		},
		"sort": map[string]any{"field": "relevance"},
	}

	// type filter: v3 API requires type=BlocksInSpace; filter results post-hoc by block type
	typeFilter := argStr(args, "type")

	if v := argInt(args, "limit"); v > 0 {
		if v > 100 {
			v = 100
		}
		body["limit"] = v
	}
	if v := argMap(args, "filters"); v != nil {
		body["filters"] = v
	}
	if v := argMap(args, "sort"); v != nil {
		body["sort"] = v
	}

	data, err := n.doRequest(ctx, "/api/v3/search", body)
	if err != nil {
		return errResult(err)
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
		return errResult(err)
	}

	// Build block lookup from recordMap
	blocks := parseBlockTable(resp.RecordMap)

	// Merge results with block data — include all fields, let compaction strip noise.
	var results []map[string]any
	for _, r := range resp.Results {
		entry := map[string]any{
			"id":        r.ID,
			"highlight": r.Highlight,
		}
		block, ok := blocks[r.ID]
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
		for _, r := range results {
			if matchesTypeFilter(r, typeFilter) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	return jsonResult(map[string]any{
		"results": results,
		"total":   resp.Total,
	})
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
//	"page"        → block type "page"
//	"data_source" → block type "collection_view_page" or "collection_view"
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
