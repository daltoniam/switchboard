package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constructor ---

func TestNew_ReturnsIntegrationNamedNotion(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "notion", i.Name())
}

// --- Configure ---

func TestConfigure_AcceptsValidCredentials(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"integration_secret": "ntn_test123"})
	assert.NoError(t, err)
}

func TestConfigure_RejectsEmptyIntegrationSecret(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"integration_secret": ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "integration_secret is required")
}

func TestConfigure_TrimsTrailingSlashFromCustomBaseURL(t *testing.T) {
	n := &notion{client: &http.Client{}}
	err := n.Configure(mcp.Credentials{"integration_secret": "key", "base_url": "https://custom.notion.com/"})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.notion.com", n.baseURL)
}

func TestConfigure_DefaultsToNotionAPIBaseURL(t *testing.T) {
	n := New().(*notion)
	err := n.Configure(mcp.Credentials{"integration_secret": "key"})
	assert.NoError(t, err)
	assert.Equal(t, "https://api.notion.com", n.baseURL)
}

// --- Tools metadata ---

func TestTools_AllHaveNameAndDescription(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllPrefixedWithNotion(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "notion_", "tool %s missing notion_ prefix", tool.Name)
	}
}

func TestTools_NamesAreUnique(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

// --- Dispatch parity ---

func TestDispatchMap_EveryToolHasHandler(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- Execute ---

func TestExecute_ReturnsErrorForUnknownTool(t *testing.T) {
	n := &notion{integrationSecret: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := n.Execute(context.Background(), "notion_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

// --- HTTP helpers ---

func TestDoRequest_ReturnsJSONOnSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc123"}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "test-key", baseURL: ts.URL, client: ts.Client()}
	data, err := n.get(context.Background(), "/v1/users/me")
	require.NoError(t, err)
	assert.Contains(t, string(data), "abc123")
}

func TestDoRequest_ReturnsErrorOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "bad-key", baseURL: ts.URL, client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/users/me")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notion API error (401)")
}

func TestDoRequest_ReturnsSuccessOn204(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := n.doRequest(context.Background(), "DELETE", "/v1/blocks/123", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_SetsNotionVersionHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "2025-09-03", r.Header.Get("Notion-Version"))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "key", baseURL: ts.URL, client: ts.Client()}
	_, err := n.get(context.Background(), "/v1/users/me")
	require.NoError(t, err)
}

func TestPost_SendsJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := n.post(context.Background(), "/v1/pages", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestPatch_SendsJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"updated":true}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := n.patch(context.Background(), "/v1/pages/123", map[string]string{"title": "new"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

func TestDel_SendsDeleteToCorrectPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/v1/blocks/abc", r.URL.Path)
		_, _ = w.Write([]byte(`{"archived":true}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := n.del(context.Background(), "/v1/blocks/%s", "abc")
	require.NoError(t, err)
	assert.Contains(t, string(data), "archived")
}

// --- Result helpers ---

func TestRawResult_WrapsDataWithoutError(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult_WrapsErrorMessage(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helpers ---

func TestArgStr_ExtractsStringValue(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt_CoercesMultipleTypes(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool_CoercesBoolAndString(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestArgMap_ExtractsNestedMap(t *testing.T) {
	inner := map[string]any{"nested": "value"}
	assert.Equal(t, inner, argMap(map[string]any{"m": inner}, "m"))
	assert.Nil(t, argMap(map[string]any{}, "m"))
	assert.Nil(t, argMap(map[string]any{"m": "not-a-map"}, "m"))
}

// --- Healthy ---

func TestHealthy_ReturnsTrueOnSuccessfulAPICall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"bot-user"}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "key", baseURL: ts.URL, client: ts.Client()}
	assert.True(t, n.Healthy(context.Background()))
}

func TestHealthy_ReturnsFalseOnAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	n := &notion{integrationSecret: "bad-key", baseURL: ts.URL, client: ts.Client()}
	assert.False(t, n.Healthy(context.Background()))
}

func TestHealthy_ReturnsFalseWhenUnconfigured(t *testing.T) {
	n := &notion{}
	assert.False(t, n.Healthy(context.Background()))
}

// --- Handler test helpers ---

func testNotion(t *testing.T, handler http.HandlerFunc) *notion {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &notion{integrationSecret: "test-key", baseURL: ts.URL, client: ts.Client()}
}

func okJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// --- Users ---

func TestListUsers_ReturnsPaginatedResults(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/v1/users", r.URL.Path)
		okJSON(w, `{"results":[{"id":"u1"}],"has_more":false}`)
	})
	result, err := listUsers(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "u1")
}

func TestListUsers_SendsPaginationParams(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "cursor123", r.URL.Query().Get("start_cursor"))
		assert.Equal(t, "10", r.URL.Query().Get("page_size"))
		okJSON(w, `{"results":[]}`)
	})
	_, err := listUsers(context.Background(), n, map[string]any{
		"start_cursor": "cursor123",
		"page_size":    float64(10),
	})
	require.NoError(t, err)
}

func TestRetrieveUser_FetchesUserByID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/users/user-abc", r.URL.Path)
		okJSON(w, `{"id":"user-abc","name":"Alice"}`)
	})
	result, err := retrieveUser(context.Background(), n, map[string]any{"user_id": "user-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "Alice")
}

func TestRetrieveUser_RequiresUserID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := retrieveUser(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "user_id is required")
}

func TestGetSelf_ReturnsBotUser(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/users/me", r.URL.Path)
		okJSON(w, `{"id":"bot-123","type":"bot"}`)
	})
	result, err := getSelf(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "bot-123")
}

// --- Search ---

func TestSearchToolDescription_DocumentsDataSourceFilter(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		if tool.Name == "notion_search" {
			assert.Contains(t, tool.Parameters["filter"], "data_source",
				"search filter description should mention data_source, not database")
			return
		}
	}
	t.Fatal("notion_search tool not found")
}

func TestSearchNotion_PostsQueryAndFilters(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/search", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "meeting notes", body["query"])
		okJSON(w, `{"results":[{"id":"p1"}]}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{
		"query": "meeting notes",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "p1")
}

func TestSearchNotion_OmitsEmptyFields(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Empty(t, body)
		okJSON(w, `{"results":[]}`)
	})
	_, err := searchNotion(context.Background(), n, map[string]any{})
	require.NoError(t, err)
}

// --- Comments ---

func TestCreateComment_PostsRichTextAndParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/comments", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["rich_text"])
		assert.NotNil(t, body["parent"])
		okJSON(w, `{"id":"comment-1"}`)
	})
	result, err := createComment(context.Background(), n, map[string]any{
		"rich_text": []any{map[string]any{"type": "text", "text": map[string]any{"content": "Hello"}}},
		"parent":    map[string]any{"page_id": "page-1"},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "comment-1")
}

func TestCreateComment_RequiresRichText(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := createComment(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "rich_text is required")
}

func TestRetrieveComments_FetchesByBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "block-abc", r.URL.Query().Get("block_id"))
		okJSON(w, `{"results":[{"id":"c1"}]}`)
	})
	result, err := retrieveComments(context.Background(), n, map[string]any{"block_id": "block-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "c1")
}

func TestRetrieveComments_RequiresBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := retrieveComments(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")
}

// --- Blocks ---

func TestRetrieveBlock_FetchesBlockByID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/blocks/blk-1", r.URL.Path)
		okJSON(w, `{"id":"blk-1","type":"paragraph"}`)
	})
	result, err := retrieveBlock(context.Background(), n, map[string]any{"block_id": "blk-1"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "paragraph")
}

func TestRetrieveBlock_RequiresBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := retrieveBlock(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")
}

func TestUpdateBlock_MergesTypeContentIntoBody(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/v1/blocks/blk-1", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["paragraph"])
		okJSON(w, `{"id":"blk-1"}`)
	})
	result, err := updateBlock(context.Background(), n, map[string]any{
		"block_id":     "blk-1",
		"type_content": map[string]any{"paragraph": map[string]any{"rich_text": []any{}}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteBlock_SendsDeleteRequest(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/v1/blocks/blk-1", r.URL.Path)
		okJSON(w, `{"archived":true}`)
	})
	result, err := deleteBlock(context.Background(), n, map[string]any{"block_id": "blk-1"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "archived")
}

func TestGetBlockChildren_FetchesChildrenWithPagination(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/blocks/blk-1/children", r.URL.Path)
		assert.Equal(t, "50", r.URL.Query().Get("page_size"))
		okJSON(w, `{"results":[{"id":"child-1"}]}`)
	})
	result, err := getBlockChildren(context.Background(), n, map[string]any{
		"block_id":  "blk-1",
		"page_size": float64(50),
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "child-1")
}

func TestAppendBlockChildren_SendsChildrenArray(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/v1/blocks/blk-1/children", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["children"])
		okJSON(w, `{"results":[]}`)
	})
	result, err := appendBlockChildren(context.Background(), n, map[string]any{
		"block_id": "blk-1",
		"children": []any{map[string]any{"type": "paragraph"}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestAppendBlockChildren_RequiresChildren(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := appendBlockChildren(context.Background(), n, map[string]any{"block_id": "blk-1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "children is required")
}

// --- Pages ---

func TestCreatePage_PostsParentAndProperties(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/pages", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		parent := body["parent"].(map[string]any)
		assert.Equal(t, "db-1", parent["database_id"])
		okJSON(w, `{"id":"page-new"}`)
	})
	result, err := createPage(context.Background(), n, map[string]any{
		"parent":     map[string]any{"database_id": "db-1"},
		"properties": map[string]any{"Name": map[string]any{"title": []any{}}},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "page-new")
}

func TestCreatePage_RequiresParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := createPage(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parent is required")
}

func TestRetrievePage_FetchesPageByID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/pages/page-abc", r.URL.Path)
		okJSON(w, `{"id":"page-abc","object":"page"}`)
	})
	result, err := retrievePage(context.Background(), n, map[string]any{"page_id": "page-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "page-abc")
}

func TestUpdatePage_PatchesProperties(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/v1/pages/page-abc", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["properties"])
		okJSON(w, `{"id":"page-abc"}`)
	})
	result, err := updatePage(context.Background(), n, map[string]any{
		"page_id":    "page-abc",
		"properties": map[string]any{"Status": map[string]any{"select": map[string]any{"name": "Done"}}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestMovePage_PostsNewParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/pages/page-abc/move", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		parent := body["parent"].(map[string]any)
		assert.Equal(t, "page-target", parent["page_id"])
		okJSON(w, `{"id":"page-abc"}`)
	})
	result, err := movePage(context.Background(), n, map[string]any{
		"page_id": "page-abc",
		"parent":  map[string]any{"page_id": "page-target"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestMovePage_RequiresPageID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := movePage(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "page_id is required")
}

func TestMovePage_RequiresParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := movePage(context.Background(), n, map[string]any{"page_id": "p1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parent is required")
}

func TestRetrievePageProperty_FetchesPropertyByID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/pages/page-1/properties/prop-1", r.URL.Path)
		okJSON(w, `{"object":"property_item","type":"title"}`)
	})
	result, err := retrievePageProperty(context.Background(), n, map[string]any{
		"page_id":     "page-1",
		"property_id": "prop-1",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "title")
}

// --- Data Sources ---

func TestCreateDatabase_PostsParentAndProperties(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/databases", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		parent := body["parent"].(map[string]any)
		assert.Equal(t, "page-1", parent["page_id"])
		okJSON(w, `{"id":"db-new"}`)
	})
	result, err := createDatabase(context.Background(), n, map[string]any{
		"parent": map[string]any{"page_id": "page-1"},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "db-new")
}

func TestCreateDatabase_RequiresParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := createDatabase(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parent is required")
}

func TestRetrieveDataSource_FetchesByID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/data_sources/ds-abc", r.URL.Path)
		okJSON(w, `{"id":"ds-abc"}`)
	})
	result, err := retrieveDataSource(context.Background(), n, map[string]any{"data_source_id": "ds-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "ds-abc")
}

func TestUpdateDataSource_PatchesTitleAndProperties(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/v1/data_sources/ds-abc", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["title"])
		assert.Nil(t, body["description"], "description is not supported by the data sources API")
		okJSON(w, `{"id":"ds-abc"}`)
	})
	result, err := updateDataSource(context.Background(), n, map[string]any{
		"data_source_id": "ds-abc",
		"title":          []any{map[string]any{"type": "text", "text": map[string]any{"content": "New Title"}}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryDataSource_PostsFilterAndSorts(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/data_sources/ds-abc/query", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["filter"])
		okJSON(w, `{"results":[{"id":"row-1"}]}`)
	})
	result, err := queryDataSource(context.Background(), n, map[string]any{
		"data_source_id": "ds-abc",
		"filter":         map[string]any{"property": "Status", "select": map[string]any{"equals": "Done"}},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "row-1")
}

func TestListDataSourceTemplates_FetchesWithPagination(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/data_sources/ds-abc/templates", r.URL.Path)
		okJSON(w, `{"results":[{"id":"tmpl-1"}]}`)
	})
	result, err := listDataSourceTemplates(context.Background(), n, map[string]any{
		"data_source_id": "ds-abc",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "tmpl-1")
}

// --- Databases ---

func TestRetrieveDatabase_FetchesByID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/databases/db-abc", r.URL.Path)
		okJSON(w, `{"id":"db-abc","object":"database"}`)
	})
	result, err := retrieveDatabase(context.Background(), n, map[string]any{"database_id": "db-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "db-abc")
}

func TestRetrieveDatabase_RequiresDatabaseID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := retrieveDatabase(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "database_id is required")
}

// --- Convenience: getPageContent ---

func TestGetPageContent_FetchesPageAndBlocksRecursively(t *testing.T) {
	callCount := 0
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/v1/pages/page-1":
			okJSON(w, `{"id":"page-1","object":"page"}`)
		case "/v1/blocks/page-1/children":
			okJSON(w, `{"results":[{"id":"blk-1","has_children":true,"type":"toggle"}],"has_more":false}`)
		case "/v1/blocks/blk-1/children":
			okJSON(w, `{"results":[{"id":"blk-2","has_children":false,"type":"paragraph"}],"has_more":false}`)
		default:
			w.WriteHeader(404)
		}
	})

	result, err := getPageContent(context.Background(), n, map[string]any{"page_id": "page-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "page-1")
	assert.Contains(t, result.Data, "blk-1")
	assert.Contains(t, result.Data, "blk-2")
	assert.Equal(t, 3, callCount)
}

func TestGetPageContent_RespectsMaxDepth(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pages/page-1":
			okJSON(w, `{"id":"page-1"}`)
		case "/v1/blocks/page-1/children":
			okJSON(w, `{"results":[{"id":"blk-1","has_children":true}],"has_more":false}`)
		default:
			t.Errorf("unexpected request to %s — max_depth should have stopped recursion", r.URL.Path)
			w.WriteHeader(404)
		}
	})

	result, err := getPageContent(context.Background(), n, map[string]any{
		"page_id":   "page-1",
		"max_depth": float64(1),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "blk-1")
}

func TestGetPageContent_RequiresPageID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := getPageContent(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "page_id is required")
}

func TestGetPageContent_IndicatesTruncationWhenFetchLimitReached(t *testing.T) {
	// Serve a page with many blocks that each have children, exhausting maxBlockFetches.
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pages/page-1":
			okJSON(w, `{"id":"page-1","object":"page"}`)
		default:
			// Every block children request returns 5 blocks with has_children:true and has_more:true,
			// which forces continued pagination + recursion until remaining hits 0.
			okJSON(w, `{"results":[
				{"id":"b1","has_children":true,"type":"paragraph"},
				{"id":"b2","has_children":true,"type":"paragraph"},
				{"id":"b3","has_children":true,"type":"paragraph"},
				{"id":"b4","has_children":true,"type":"paragraph"},
				{"id":"b5","has_children":true,"type":"paragraph"}
			],"has_more":true,"next_cursor":"cursor-next"}`)
		}
	})

	result, err := getPageContent(context.Background(), n, map[string]any{
		"page_id":   "page-1",
		"max_depth": float64(3),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"truncated":true`)
}

func TestGetPageContent_NoTruncationFieldWhenComplete(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pages/page-1":
			okJSON(w, `{"id":"page-1","object":"page"}`)
		case "/v1/blocks/page-1/children":
			okJSON(w, `{"results":[{"id":"blk-1","has_children":false,"type":"paragraph"}],"has_more":false}`)
		default:
			w.WriteHeader(404)
		}
	})

	result, err := getPageContent(context.Background(), n, map[string]any{"page_id": "page-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.NotContains(t, result.Data, "truncated")
}

// --- Convenience: createPageWithContent ---

func TestCreatePageWithContent_SendsChildrenInBody(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/pages", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["parent"])
		assert.NotNil(t, body["children"])
		okJSON(w, `{"id":"page-new"}`)
	})
	result, err := createPageWithContent(context.Background(), n, map[string]any{
		"parent":   map[string]any{"page_id": "parent-1"},
		"children": []any{map[string]any{"type": "paragraph"}},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "page-new")
}

func TestCreatePageWithContent_RequiresParentAndChildren(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	result, err := createPageWithContent(context.Background(), n, map[string]any{
		"parent": map[string]any{"page_id": "p1"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "children is required")
}
