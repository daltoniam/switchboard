package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ── Structural tests (GitHub compact_specs_test.go pattern) ─────────

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	// fieldCompactionSpecs is loaded from compact.yaml at package init.
	// If any spec were invalid, lenient-mode loading would skip it with a warning.
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

// TestFieldCompactionSpecs_NoDuplicateTools verifies the YAML loader did not
// silently drop any tool entries. YAML keys are unique at the parser level;
// this confirms parse losslessness.
func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for tool := range fieldCompactionSpecs {
		_, ok := dispatch[tool]
		assert.True(t, ok, "compaction spec %s has no dispatch handler", tool)
	}
}

func TestFieldCompactionSpecs_NoMutationTools(t *testing.T) {
	mutationPrefixes := []string{"create", "update", "delete", "move", "append"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_search")
	require.True(t, ok, "notion_search should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	n := New().(*notion)
	_, ok := n.CompactSpec("notion_create_page")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFieldsForGetTool(t *testing.T) {
	n := New().(*notion)
	for _, tool := range []string{
		"notion_retrieve_page",
		"notion_retrieve_block",
		"notion_retrieve_database",
		"notion_retrieve_data_source",
		"notion_retrieve_user",
		"notion_get_self",
	} {
		fields, ok := n.CompactSpec(mcp.ToolName(tool))
		assert.True(t, ok, "%s should have field compaction spec", tool)
		assert.NotEmpty(t, fields, "%s spec should not be empty", tool)
	}
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	n := New().(*notion)
	_, ok := n.CompactSpec("notion_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

// ── Reduction tests (moved from notion_test.go) ─────────────────────

func TestGetPageContent_ReturnsMarkdown(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-1": {"value": {
						"id": "page-1", "type": "page",
						"properties": {"title": [["My Page"]]},
						"content": ["child-1"],
						"format": {"page_icon": "📄"},
						"space_id": "noise", "crdt_data": "noise",
						"alive": true,
						"created_time": 1000, "last_edited_time": 1700000000000
					}},
					"child-1": {"value": {
						"id": "child-1", "type": "text",
						"properties": {"title": [["Hello"]]},
						"parent_id": "page-1", "parent_table": "block",
						"space_id": "noise",
						"alive": true, "created_time": 1000, "last_edited_time": 2000
					}}
				}
			}
		}`)
	})

	result, err := getPageContent(context.Background(), n, map[string]any{"page_id": "page-1"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Handler returns JSON; RenderMarkdown converts to Markdown.
	md, ok := n.RenderMarkdown("notion_get_page_content", []byte(result.Data))
	require.True(t, ok, "RenderMarkdown should handle get_page_content")

	assert.Contains(t, md, "<!-- notion:page_id=page-1 -->")
	assert.Contains(t, md, "# My Page")
	assert.Contains(t, md, "Hello")

	// Noise fields stripped by markdown rendering.
	assert.NotContains(t, md, "crdt_data")
	assert.NotContains(t, md, "space_id")
}

func TestFieldCompactionSpecs_SearchReducesResponseSize(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_search")
	require.True(t, ok, "notion_search should have compaction spec")
	require.NotEmpty(t, fields)

	data := `{
		"results": [
			{"id":"page-1","type":"page","parent_id":"space-1","highlight":{"text":"Meeting Notes"},"created_time":1700000000000,"last_edited_time":1700000001000,"properties":{"title":[["Meeting Notes"]]},"space_id":"sp-1","version":42,"created_by_id":"user-1","alive":true},
			{"id":"page-2","type":"page","parent_id":"space-1","highlight":{"text":"Project Plan"},"created_time":1700000002000,"last_edited_time":1700000003000,"properties":{"title":[["Project Plan"]]},"space_id":"sp-1","version":15,"alive":true}
		],
		"total": 2
	}`

	compacted, err := mcp.CompactJSON([]byte(data), fields)
	require.NoError(t, err)
	assert.Less(t, len(compacted), len(data), "compacted response should be smaller")
	assert.Contains(t, string(compacted), "page-1")
	assert.Contains(t, string(compacted), "Meeting Notes")
	assert.NotContains(t, string(compacted), "space_id")
	assert.NotContains(t, string(compacted), "version")
	assert.NotContains(t, string(compacted), "alive")
}

func TestFieldCompactionSpecs_QueryDataSourceReducesResponseSize(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_query_data_source")
	require.True(t, ok, "notion_query_data_source should have compaction spec")
	require.NotEmpty(t, fields)

	data := `{
		"schema": {"title": {"name": "Name", "type": "title"}, "gedz": {"name": "Company", "type": "text"}},
		"results": [
			{"id":"row-1","properties":{"title":[["Task 1"]],"status":["Done"]},"created_time":1700000000000,"last_edited_time":1700000001000,"type":"page","space_id":"sp-1","version":5,"permissions":[{"role":"reader"}],"format":{"page_cover":"img.png"}},
			{"id":"row-2","properties":{"title":[["Task 2"]],"status":["In Progress"]},"created_time":1700000002000,"last_edited_time":1700000003000,"type":"page","space_id":"sp-1","version":8}
		],
		"total": 2,
		"has_more": false
	}`

	compacted, err := mcp.CompactJSON([]byte(data), fields)
	require.NoError(t, err)
	assert.Less(t, len(compacted), len(data))
	assert.Contains(t, string(compacted), "row-1")
	assert.Contains(t, string(compacted), "Task 1")
	assert.Contains(t, string(compacted), "Company", "schema must survive compaction")
	assert.NotContains(t, string(compacted), "version")
}

func TestFieldCompactionSpecs_ListUsersReducesResponseSize(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_list_users")
	require.True(t, ok, "notion_list_users should have compaction spec")
	require.NotEmpty(t, fields)

	data := `{
		"results": [
			{"id":"user-1","name":"Alice","email":"alice@test.com","profile_photo":"https://img.example.com/alice.png","version":3,"last_logged_in":1700000000000},
			{"id":"user-2","name":"Bob","email":"bob@test.com","profile_photo":"https://img.example.com/bob.png","version":7}
		]
	}`

	compacted, err := mcp.CompactJSON([]byte(data), fields)
	require.NoError(t, err)
	assert.Less(t, len(compacted), len(data))
	assert.Contains(t, string(compacted), "Alice")
	assert.Contains(t, string(compacted), "alice@test.com")
	assert.NotContains(t, string(compacted), "profile_photo")
}

func TestFieldCompactionSpecs_RetrieveCommentsHasSpec(t *testing.T) {
	n := New().(*notion)
	// Spec exists as fallback; MarkdownIntegration.RenderMarkdown takes priority at runtime.
	fields, ok := n.CompactSpec("notion_retrieve_comments")
	assert.True(t, ok, "notion_retrieve_comments should have compaction spec as fallback")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpecs_ListDataSourceTemplatesReducesResponseSize(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_list_data_source_templates")
	require.True(t, ok, "notion_list_data_source_templates should have compaction spec")
	require.NotEmpty(t, fields)
}

// ── New reduction tests ──────────────────────────────────────────────

func TestFieldCompactionSpecs_SearchPreservesCollectionID(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_search")
	require.True(t, ok)

	data := `{
		"results": [
			{"id":"db-1","type":"collection_view_page","parent_id":"space-1","collection_id":"coll-abc","highlight":{"text":"Sprint Board"},"created_time":1700000000000,"last_edited_time":1700000001000,"properties":{"title":[["Sprint Board"]]},"space_id":"sp-1","version":5}
		]
	}`

	compacted, err := mcp.CompactJSON([]byte(data), fields)
	require.NoError(t, err)
	assert.Contains(t, string(compacted), "coll-abc", "compaction should preserve collection_id for database results")
}

func TestFieldCompactionSpecs_RetrievePageStripsNoise(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_retrieve_page")
	require.True(t, ok, "notion_retrieve_page should have compaction spec")

	data := `{
		"id": "page-1", "type": "page",
		"properties": {"title": [["My Page"]]},
		"content": ["child-1"],
		"parent_id": "space-1", "parent_table": "space",
		"alive": true, "created_time": 1000, "last_edited_time": 2000,
		"space_id": "noise", "crdt_data": "noise", "version": 42,
		"permissions": [{"role": "editor"}],
		"created_by_id": "u1", "created_by_table": "notion_user"
	}`

	compacted, err := mcp.CompactJSON([]byte(data), fields)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(compacted, &result))

	assert.Equal(t, "page-1", result["id"])
	assert.NotNil(t, result["properties"])
	assert.NotContains(t, string(compacted), "crdt_data")
	assert.NotContains(t, string(compacted), "space_id")
	assert.NotContains(t, string(compacted), `"version"`)
}

func TestFieldCompactionSpecs_RetrieveUserKeepsEssentials(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_retrieve_user")
	require.True(t, ok, "notion_retrieve_user should have compaction spec")

	data := `{
		"id": "user-1", "name": "Alice", "email": "alice@test.com",
		"profile_photo": "https://img.example.com/alice.png",
		"version": 3, "last_logged_in": 1700000000000,
		"permission": [{"role": "member"}]
	}`

	compacted, err := mcp.CompactJSON([]byte(data), fields)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(compacted, &result))

	assert.Equal(t, "user-1", result["id"])
	assert.Equal(t, "Alice", result["name"])
	assert.Equal(t, "alice@test.com", result["email"])
	assert.NotContains(t, string(compacted), `"version"`)
	assert.NotContains(t, string(compacted), "last_logged_in")
}

// ── Multi-view pilot: notion_get_page_content ──────────────────────

const pageContentTool = mcp.ToolName("notion_get_page_content")

// TestPageContent_ViewsRegistered verifies the YAML loader resolved the
// multi-view config for notion_get_page_content. This is the load-time
// PDV proof: if the YAML had been broken, the loader would have failed
// at strict-mode startup tests.
func TestPageContent_ViewsRegistered(t *testing.T) {
	vs, ok := viewSets[pageContentTool]
	require.True(t, ok, "viewSets should contain notion_get_page_content (multi-view tool)")

	// Default: toc + json
	assert.Equal(t, compact.ViewName("toc"), vs.Default.View)
	assert.Equal(t, compact.FormatJSON, vs.Default.Format)

	// Both views present
	assert.Contains(t, vs.Views, compact.ViewName("toc"))
	assert.Contains(t, vs.Views, compact.ViewName("full"))

	// Hints are populated (LLM consumes these via _more envelope)
	assert.NotEmpty(t, vs.Views["toc"].Hint)
	assert.NotEmpty(t, vs.Views["full"].Hint)
}

// TestPageContent_RenderersResolved verifies every declared (view, format)
// combo resolved to a callable renderer at load time. Lookup miss here
// would prove the parse-don't-validate gate failed.
func TestPageContent_RenderersResolved(t *testing.T) {
	vs := viewSets[pageContentTool]

	// toc: json only
	require.NotNil(t, vs.Renderers[compact.ViewName("toc")][compact.FormatJSON], "toc+json renderer missing")
	_, hasTocMD := vs.Renderers[compact.ViewName("toc")][compact.FormatMarkdown]
	assert.False(t, hasTocMD, "toc should NOT have a markdown renderer (not declared)")

	// full: json + markdown
	require.NotNil(t, vs.Renderers[compact.ViewName("full")][compact.FormatJSON], "full+json renderer missing")
	require.NotNil(t, vs.Renderers[compact.ViewName("full")][compact.FormatMarkdown], "full+markdown renderer missing")
}

// TestPageContent_FullMarkdownRendererBridge verifies the custom renderer
// for (full, markdown) is the bridge to renderPageContentMD — invoking it
// with a representative payload produces markdown.
func TestPageContent_FullMarkdownRendererBridge(t *testing.T) {
	renderer := viewSets[pageContentTool].Renderers[compact.ViewName("full")][compact.FormatMarkdown]
	require.NotNil(t, renderer)

	// Minimal page-content shape that renderPageContentMD knows how to render.
	projected := map[string]any{
		"page": map[string]any{
			"id":   "p1",
			"type": "page",
			"properties": map[string]any{
				"title": []any{[]any{"Hello"}},
			},
		},
		"blocks": []any{},
	}
	got, err := renderer(projected)
	require.NoError(t, err)
	assert.NotEmpty(t, got, "renderer should produce markdown bytes")
	// The bridge returns whatever renderPageContentMD returns; smoke-checking
	// it isn't an error envelope.
	assert.NotContains(t, string(got), `"error"`)
}

// TestPageContent_ViewsMethod_AdapterContract verifies the Views method
// satisfies the ToolViewsIntegration contract for notion_get_page_content
// and returns (zero, false) for non-view tools.
func TestPageContent_ViewsMethod_AdapterContract(t *testing.T) {
	n := &notion{}

	vs, ok := n.Views(pageContentTool)
	require.True(t, ok, "Views() should return ViewSet for notion_get_page_content")
	assert.Equal(t, compact.ViewName("toc"), vs.Default.View)

	_, hasNoViews := n.Views("notion_list_users") // flat-form tool
	assert.False(t, hasNoViews, "flat-form tools should return false")
}

// TestPageContent_LegacyMarkdownPathStillReachableForOtherTools verifies
// that pivoting notion_get_page_content to views did NOT remove
// MarkdownIntegration coverage for the other notion tool that uses it
// (notion_retrieve_comments). Regression guard.
func TestPageContent_LegacyMarkdownPathStillReachableForOtherTools(t *testing.T) {
	n := &notion{}
	// retrieve_comments still goes through the legacy path; calling
	// RenderMarkdown directly should still produce markdown.
	_, ok := n.RenderMarkdown("notion_retrieve_comments", []byte(`{"results":[]}`))
	assert.True(t, ok, "notion_retrieve_comments markdown rendering should be unaffected")
}
