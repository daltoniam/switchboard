package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Structural tests (GitHub compact_specs_test.go pattern) ─────────

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	// fieldCompactionSpecs is built via mustBuildFieldCompactionSpecs at package init.
	// If any spec were invalid, the package would panic on load.
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	assert.Equal(t, len(rawFieldCompactionSpecs), len(fieldCompactionSpecs))
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
		fields, ok := n.CompactSpec(tool)
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

func TestGetPageContent_CompactionStripsBlockNoise(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-1": {"value": {
						"id": "page-1", "type": "page",
						"properties": {"title": [["My Page"]]},
						"content": ["child-1"],
						"format": {"page_icon": "📄", "copied_from_pointer": {"id": "tmpl-1"}},
						"space_id": "noise", "crdt_data": "noise",
						"permissions": [{"role": "editor"}],
						"created_by_id": "u1", "created_by_table": "notion_user",
						"last_edited_by_id": "u1", "last_edited_by_table": "notion_user",
						"version": 42, "alive": true,
						"created_time": 1000, "last_edited_time": 2000
					}},
					"child-1": {"value": {
						"id": "child-1", "type": "text",
						"properties": {"title": [["Hello"]]},
						"parent_id": "page-1", "parent_table": "block",
						"space_id": "noise", "crdt_data": "noise",
						"copied_from": "some-block",
						"permissions": [{"role": "reader"}],
						"created_by_id": "u1", "created_by_table": "notion_user",
						"version": 7, "ignore_block_count": true,
						"alive": true, "created_time": 1000, "last_edited_time": 2000
					}}
				}
			}
		}`)
	})

	result, err := getPageContent(context.Background(), n, map[string]any{"page_id": "page-1"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Apply compaction as the server would
	adapter := New().(*notion)
	fields, ok := adapter.CompactSpec("notion_get_page_content")
	require.True(t, ok, "get_page_content must have a compaction spec")

	compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(compacted, &resp))

	// Page compacted — essential fields preserved, noise stripped
	page := resp["page"].(map[string]any)
	assert.Equal(t, "page-1", page["id"])
	assert.Equal(t, "page", page["type"])
	assert.NotNil(t, page["properties"])
	assert.NotNil(t, page["content"])
	assert.NotNil(t, page["format"])

	// Page noise fields stripped by compaction
	for _, key := range []string{"space_id", "crdt_data", "permissions", "created_by_id", "created_by_table", "last_edited_by_id", "last_edited_by_table", "version"} {
		_, present := page[key]
		assert.False(t, present, "compaction should strip %q from page", key)
	}

	// Blocks compacted — only listed fields survive
	blocks := resp["blocks"].([]any)
	require.Len(t, blocks, 1)
	child := blocks[0].(map[string]any)
	assert.Equal(t, "child-1", child["id"])
	assert.Equal(t, "text", child["type"])
	assert.Equal(t, "page-1", child["parent_id"])

	// Noise fields stripped by compaction
	for _, key := range []string{"space_id", "crdt_data", "copied_from", "permissions", "created_by_id", "version", "ignore_block_count"} {
		_, present := child[key]
		assert.False(t, present, "compaction should strip %q from blocks", key)
	}
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

func TestFieldCompactionSpecs_RetrieveCommentsReducesResponseSize(t *testing.T) {
	n := New().(*notion)
	fields, ok := n.CompactSpec("notion_retrieve_comments")
	require.True(t, ok, "notion_retrieve_comments should have compaction spec")
	require.NotEmpty(t, fields)
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
