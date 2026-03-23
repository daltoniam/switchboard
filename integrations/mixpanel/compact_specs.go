package mixpanel

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	// All Mixpanel Query API endpoints return JSON objects with varying structures.
	// All tools are read-only queries.

	// ── Insights ────────────────────────────────────────────────────
	// Returns the computed results of a saved report
	"mixpanel_query_insights": {"results", "series", "date_range"},

	// ── Funnels ─────────────────────────────────────────────────────
	// Returns {"meta": {...}, "data": {"<date>": {"steps": [...], "analysis": {...}}}}
	"mixpanel_query_funnels": {"meta", "data"},

	// ── Retention ───────────────────────────────────────────────────
	// Returns {"results": {...}} with cohort retention data
	"mixpanel_query_retention": {"results"},

	// ── Segmentation ────────────────────────────────────────────────
	// Returns {"data": {"series": [...], "values": {...}}, "legend_size": N}
	"mixpanel_query_segmentation": {"data.series", "data.values", "legend_size"},

	// ── Event Properties ────────────────────────────────────────────
	// Returns {"data": {"series": [...], "values": {...}}, "legend_size": N}
	"mixpanel_query_event_properties": {"data.series", "data.values", "legend_size"},

	// ── Profiles (Engage) ───────────────────────────────────────────
	// Returns {"page", "page_size", "session_id", "status", "total", "results": [...]}
	"mixpanel_query_profiles": {
		"results[].$distinct_id",
		"results[].$properties",
		"page",
		"page_size",
		"session_id",
		"status",
		"total",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("mixpanel: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
