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

// TestGetPageContent_ReturnsMarkdown drives the full views pipeline:
// handler → JSON → CompactAny(full spec) → custom markdown renderer.
// Production processResult does these steps in order; the test asserts
// the integrated result. RenderMarkdown is no longer in the picture for
// view-aware tools, so the old direct-RenderMarkdown invocation was
// stale — replaced with the actual production path.
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

	// Production path: JSON → CompactAny(full view spec) → renderer.
	var parsed any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &parsed))
	fullSpec := viewSets[pageContentTool].Views[compact.ViewName("full")].Spec
	projected := mcp.CompactAny(parsed, fullSpec)
	renderer := viewSets[pageContentTool].Renderers[compact.ViewName("full")][compact.FormatMarkdown]
	require.NotNil(t, renderer)
	md, err := renderer(projected)
	require.NoError(t, err)

	out := string(md)
	assert.Contains(t, out, "<!-- notion:page_id=page-1 -->")
	assert.Contains(t, out, "# My Page")
	assert.Contains(t, out, "Hello")
	// CompactAny strips noise fields before the renderer sees them.
	assert.NotContains(t, out, "crdt_data")
	assert.NotContains(t, out, "space_id")
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

// ── Views expansion: search, query, comments, retrieve_data_source ──
//
// These tests follow the same shape as the pageContent block above:
// (a) views registered with correct default + hints, (b) every declared
// (view, format) combo resolved to a callable renderer at load time.
// The PDV proof is at load time — the loader rejects unresolved combos
// in strict mode, so failures here surface as load failures, not runtime
// errors.

const (
	searchTool     = mcp.ToolName("notion_search")
	queryTool      = mcp.ToolName("notion_query_data_source")
	commentsTool   = mcp.ToolName("notion_retrieve_comments")
	dataSourceTool = mcp.ToolName("notion_retrieve_data_source")
)

// notion_search: titles (default) / full. Markdown table on titles only.
// LLMs typically scan many results to pick one, so the slim shape (links
// + titles) is the conservative default.
func TestSearch_ViewsRegistered(t *testing.T) {
	vs, ok := viewSets[searchTool]
	require.True(t, ok)

	assert.Equal(t, compact.ViewName("titles"), vs.Default.View)
	assert.Equal(t, compact.FormatJSON, vs.Default.Format)

	assert.Contains(t, vs.Views, compact.ViewName("titles"))
	assert.Contains(t, vs.Views, compact.ViewName("full"))
	assert.NotEmpty(t, vs.Views["titles"].Hint)
	assert.NotEmpty(t, vs.Views["full"].Hint)
}

func TestSearch_RenderersResolved(t *testing.T) {
	vs := viewSets[searchTool]
	require.NotNil(t, vs.Renderers[compact.ViewName("titles")][compact.FormatJSON])
	require.NotNil(t, vs.Renderers[compact.ViewName("titles")][compact.FormatMarkdown],
		"titles+markdown renderer must be registered for the link-table format")
	require.NotNil(t, vs.Renderers[compact.ViewName("full")][compact.FormatJSON])

	_, hasFullMD := vs.Renderers[compact.ViewName("full")][compact.FormatMarkdown]
	assert.False(t, hasFullMD, "full view should NOT have markdown (not declared)")
}

// renderSearchTitlesMD must produce a markdown table with one row per
// result. Failure shape: an empty body or unstructured prose would
// degrade the discovery-by-browsing use case the titles view targets.
func TestSearch_TitlesMarkdownRenderer(t *testing.T) {
	renderer := viewSets[searchTool].Renderers[compact.ViewName("titles")][compact.FormatMarkdown]
	require.NotNil(t, renderer)

	projected := map[string]any{
		"results": []any{
			// Resolved block: properties.title is populated.
			map[string]any{
				"id":   "page-1",
				"type": "page",
				"properties": map[string]any{
					"title": []any{[]any{"Meeting Notes"}},
				},
				"url": "https://www.notion.so/page-1",
			},
			// Resolved block + database: collection_id appears.
			map[string]any{
				"id":            "db-1",
				"type":          "collection_view_page",
				"collection_id": "coll-abc",
				"properties": map[string]any{
					"title": []any{[]any{"Sprint Board"}},
				},
				"url": "https://www.notion.so/db-1",
			},
			// Unresolved block: properties is empty; title falls back to
			// highlight.title (the path the renderer must support).
			map[string]any{
				"id": "page-2",
				"highlight": map[string]any{
					"title": "Unresolved Page",
					"text":  "matched snippet",
				},
				"url": "https://www.notion.so/page-2",
			},
		},
	}
	got, err := renderer(projected)
	require.NoError(t, err)
	out := string(got)
	assert.Contains(t, out, "Meeting Notes", "properties.title must appear")
	assert.Contains(t, out, "Sprint Board", "properties.title must appear")
	assert.Contains(t, out, "Unresolved Page", "highlight.title must appear when properties.title is empty")
	assert.Contains(t, out, "| ---", "must produce a markdown table separator row")
	assert.Contains(t, out, "page-1", "id must appear so LLM can drill in")
}

// notion_query_data_source: summary (default) / full.
// Summary view drops per-row non-title properties; LLM browses, then
// drills into specific rows with view=full.
func TestQuery_ViewsRegistered(t *testing.T) {
	vs, ok := viewSets[queryTool]
	require.True(t, ok)

	assert.Equal(t, compact.ViewName("summary"), vs.Default.View)
	assert.Equal(t, compact.FormatJSON, vs.Default.Format)

	assert.Contains(t, vs.Views, compact.ViewName("summary"))
	assert.Contains(t, vs.Views, compact.ViewName("full"))
	assert.NotEmpty(t, vs.Views["summary"].Hint)
	assert.NotEmpty(t, vs.Views["full"].Hint)
}

func TestQuery_RenderersResolved(t *testing.T) {
	vs := viewSets[queryTool]
	require.NotNil(t, vs.Renderers[compact.ViewName("summary")][compact.FormatJSON])
	require.NotNil(t, vs.Renderers[compact.ViewName("summary")][compact.FormatMarkdown])
	require.NotNil(t, vs.Renderers[compact.ViewName("full")][compact.FormatJSON])

	_, hasFullMD := vs.Renderers[compact.ViewName("full")][compact.FormatMarkdown]
	assert.False(t, hasFullMD)
}

// renderQuerySummaryMD must produce a row-per-record markdown table.
// Schema row should surface what columns exist; that's the LLM's
// disambiguation aid when picking which row to view in full.
func TestQuery_SummaryMarkdownRenderer(t *testing.T) {
	renderer := viewSets[queryTool].Renderers[compact.ViewName("summary")][compact.FormatMarkdown]
	require.NotNil(t, renderer)

	projected := map[string]any{
		"schema": map[string]any{
			"title": map[string]any{"name": "Name", "type": "title"},
		},
		"results": []any{
			map[string]any{
				"id": "row-1",
				"properties": map[string]any{
					"title": []any{[]any{"Task 1"}},
				},
				"last_edited_time": float64(1700000001000),
			},
		},
	}
	got, err := renderer(projected)
	require.NoError(t, err)
	out := string(got)
	assert.Contains(t, out, "Task 1")
	assert.Contains(t, out, "row-1")
	assert.Contains(t, out, "| ---", "must produce a markdown table separator row")
}

// notion_retrieve_comments: topics (default) / full.
// Topics drops author/timestamp/parent metadata, keeps the conversation
// text. Full keeps the existing rich-markdown rendering of threads.
// MarkdownIntegration for this tool moves to a registered custom
// renderer so the views pipeline owns the rendering path.
func TestComments_ViewsRegistered(t *testing.T) {
	vs, ok := viewSets[commentsTool]
	require.True(t, ok)

	assert.Equal(t, compact.ViewName("topics"), vs.Default.View)
	assert.Equal(t, compact.FormatJSON, vs.Default.Format)

	assert.Contains(t, vs.Views, compact.ViewName("topics"))
	assert.Contains(t, vs.Views, compact.ViewName("full"))
	assert.NotEmpty(t, vs.Views["topics"].Hint)
	assert.NotEmpty(t, vs.Views["full"].Hint)
}

func TestComments_RenderersResolved(t *testing.T) {
	vs := viewSets[commentsTool]
	require.NotNil(t, vs.Renderers[compact.ViewName("topics")][compact.FormatJSON])
	require.NotNil(t, vs.Renderers[compact.ViewName("full")][compact.FormatJSON])
	require.NotNil(t, vs.Renderers[compact.ViewName("full")][compact.FormatMarkdown],
		"full+markdown bridges to the existing renderCommentsMD")

	_, hasTopicsMD := vs.Renderers[compact.ViewName("topics")][compact.FormatMarkdown]
	assert.False(t, hasTopicsMD, "topics view does NOT declare markdown")
}

// renderFullCommentsMD must bridge into renderCommentsMD so the typed
// renderer registry preserves the existing rich-markdown behavior.
// Failure shape: empty bytes would silently lose markdown coverage when
// views took over.
func TestComments_FullMarkdownRendererBridge(t *testing.T) {
	renderer := viewSets[commentsTool].Renderers[compact.ViewName("full")][compact.FormatMarkdown]
	require.NotNil(t, renderer)

	projected := map[string]any{
		"results": []any{
			map[string]any{
				"discussion": map[string]any{
					"id":       "disc-1",
					"resolved": false,
				},
				"comments": []any{
					map[string]any{
						"created_by_id": "user-1",
						"created_time":  float64(1700000000000),
						"text":          []any{[]any{"first message"}},
					},
				},
			},
		},
	}
	got, err := renderer(projected)
	require.NoError(t, err)
	assert.NotEmpty(t, got)
	assert.Contains(t, string(got), "first message")
}

// Regression: once retrieve_comments moves to views, the legacy
// markdownRenderers map entry for it must be removed so a single
// pipeline owns the rendering. Two paths to the same output is the
// drift hazard the views feature was meant to eliminate.
func TestComments_LegacyMarkdownPathRemoved(t *testing.T) {
	n := &notion{}
	_, ok := n.RenderMarkdown(commentsTool, []byte(`{"results":[]}`))
	assert.False(t, ok,
		"retrieve_comments must not also register via MarkdownIntegration (views pipeline owns it)")
}

// notion_retrieve_data_source: summary / full (FULL is default).
// Inverted default — schema-on-by-default is what the LLM almost always
// wants next (query_data_source needs it). summary exists for the rare
// "does this DB exist?" case. This case is the inspiration value:
// it shows the slim view doesn't have to be the default.
func TestDataSource_ViewsRegistered(t *testing.T) {
	vs, ok := viewSets[dataSourceTool]
	require.True(t, ok)

	assert.Equal(t, compact.ViewName("full"), vs.Default.View, "default is FULL — schema-on-by-default")
	assert.Equal(t, compact.FormatJSON, vs.Default.Format)

	assert.Contains(t, vs.Views, compact.ViewName("summary"))
	assert.Contains(t, vs.Views, compact.ViewName("full"))
	assert.NotEmpty(t, vs.Views["summary"].Hint)
	assert.NotEmpty(t, vs.Views["full"].Hint)
}

func TestDataSource_RenderersResolved(t *testing.T) {
	vs := viewSets[dataSourceTool]
	require.NotNil(t, vs.Renderers[compact.ViewName("summary")][compact.FormatJSON])
	require.NotNil(t, vs.Renderers[compact.ViewName("full")][compact.FormatJSON])

	_, hasSummaryMD := vs.Renderers[compact.ViewName("summary")][compact.FormatMarkdown]
	assert.False(t, hasSummaryMD, "schema-shaped data reads fine as JSON")

	_, hasFullMD := vs.Renderers[compact.ViewName("full")][compact.FormatMarkdown]
	assert.False(t, hasFullMD)
}
