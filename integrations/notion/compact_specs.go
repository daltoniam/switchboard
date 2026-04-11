package notion

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// prefixFields wraps field specs with a prefix for compaction targeting.
// e.g. prefixFields("results[]", fields) → "results[].id", "results[].type", ...
func prefixFields(prefix string, fields []string) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = prefix + "." + f
	}
	return out
}

// ── Shared field slices ──────────────────────────────────────────────

var searchResultFields = []string{
	"id", "type", "parent_id", "collection_id",
	"properties", "highlight", "url",
	"created_time", "last_edited_time",
}

var queryResultFields = []string{
	"id", "properties",
	"created_time", "last_edited_time",
}

var userFields = []string{
	"id", "name", "email",
}

// singleUserFields includes profile_photo — useful in single-user context,
// noise in list context.
var singleUserFields = []string{
	"id", "name", "email", "profile_photo",
}

var commentFields = []string{
	"discussion",
	"comments",
}

var templateFields = []string{
	"id", "properties",
}

// dataSourceFields are the useful fields for single data source/database retrieval.
// Keeps schema (the column definitions LLMs need to construct queries).
var dataSourceFields = []string{
	"id", "name", "schema", "parent_id", "alive",
}

// blockFields are the useful fields per block in page/block list responses.
// Strips: crdt_data, crdt_format_version, copied_from, permissions,
// created_by_*, last_edited_by_*, version, ignore_block_count, space_id.
var blockFields = []string{
	"id", "type", "parent_id", "parent_table",
	"properties", "content", "format",
	"alive", "created_time", "last_edited_time",
}

// rawFieldCompactionSpecs maps tool names to dot-notation field compaction specs.
// All read tools get specs — strips CRDT noise (crdt_data, version, space_id, etc.).
// Mutation tools return small confirmation objects — no spec needed.
//
// These specs must let the LLM answer common Notion questions without extra calls:
//   - "Find pages about X" → needs id, type, highlight, parent_id, collection_id
//   - "What tasks are in this database?" → needs id, properties (title, status, assignee)
//   - "Who's in this workspace?" → needs id, name, email
//   - "What comments are on this page?" → needs discussion + comment text/author
//   - "What templates does this database have?" → needs id, properties.title
var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── List/search tools ────────────────────────────────────────────
	mcp.ToolName("notion_search"): prefixFields("results[]", searchResultFields),
	mcp.ToolName("notion_query_data_source"): append(
		[]string{"schema"},
		prefixFields("results[]", queryResultFields)...,
	),
	mcp.ToolName("notion_list_users"):                 prefixFields("results[]", userFields),
	mcp.ToolName("notion_retrieve_comments"):          prefixFields("results[]", commentFields),
	mcp.ToolName("notion_list_data_source_templates"): prefixFields("results[]", templateFields),

	// ── Composite read tools ─────────────────────────────────────────
	// get_page_content and retrieve_comments are rendered as Markdown by
	// MarkdownIntegration.RenderMarkdown — these specs serve as fallback
	// if markdown rendering is bypassed.
	mcp.ToolName("notion_get_page_content"): append(
		prefixFields("page", blockFields),
		prefixFields("blocks[]", blockFields)...,
	),

	// get_block_children returns {results: [...]}, same block noise.
	mcp.ToolName("notion_get_block_children"): prefixFields("results[]", blockFields),

	// ── Single-record get tools ──────────────────────────────────────
	mcp.ToolName("notion_retrieve_page"):        blockFields,
	mcp.ToolName("notion_retrieve_block"):       blockFields,
	mcp.ToolName("notion_retrieve_database"):    dataSourceFields,
	mcp.ToolName("notion_retrieve_data_source"): dataSourceFields,
	mcp.ToolName("notion_retrieve_user"):        singleUserFields,
	mcp.ToolName("notion_get_self"):             singleUserFields,
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("notion: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
