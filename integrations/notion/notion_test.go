package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestNew_ClientHasTimeout(t *testing.T) {
	n := New().(*notion)
	assert.Equal(t, 30*time.Second, n.client.Timeout, "http.Client must have a timeout to prevent hanging on stalled servers")
}

func TestNew_SetsMaxConnsPerHost(t *testing.T) {
	n := New().(*notion)
	transport, ok := n.client.Transport.(*http.Transport)
	require.True(t, ok, "client.Transport should be *http.Transport")
	assert.Equal(t, 10, transport.MaxConnsPerHost, "should limit concurrent connections per host")
}

func TestDoRequest_DoesNotFollowRedirects(t *testing.T) {
	redirectHit := false
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectHit = true
		if cookie := r.Header.Get("Cookie"); cookie != "" {
			t.Errorf("token leaked to redirect target: %s", cookie)
		}
		okJSON(w, `{}`)
	}))
	defer redirectTarget.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	t.Cleanup(ts.Close)

	// Use production client config (with CheckRedirect) pointed at test server
	n := New().(*notion)
	n.tokenV2 = "secret-token"
	n.spaceID = "space-1"
	n.userID = "user-1"
	n.baseURL = ts.URL

	_, _ = n.doRequest(context.Background(), "/api/v3/test", map[string]any{})
	assert.False(t, redirectHit, "redirect target should never be reached — token stays safe")
}

// --- Configure ---

func TestConfigure_AcceptsTokenV2AndResolvesSpaceAndUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/getSpaces", r.URL.Path)
		okJSON(w, `{
			"user-abc": {
				"space": {
					"space-xyz": {"value": {"id": "space-xyz"}}
				}
			}
		}`)
	}))
	defer ts.Close()

	n := &notion{client: ts.Client(), baseURL: ts.URL}
	err := n.Configure(context.Background(), mcp.Credentials{"token_v2": "v2-token-123"})
	require.NoError(t, err)
	assert.Equal(t, "v2-token-123", n.tokenV2)
	assert.Equal(t, "space-xyz", n.spaceID)
	assert.Equal(t, "user-abc", n.userID)
}

func TestConfigure_RejectsEmptyTokenV2(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"token_v2": ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token_v2 is required")
}

func TestConfigure_TrimsTrailingSlashFromCustomBaseURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{"user-1": {"space": {"sp-1": {"value": {"id": "sp-1"}}}}}`)
	}))
	defer ts.Close()

	n := &notion{client: ts.Client(), baseURL: ts.URL + "/"}
	err := n.Configure(context.Background(), mcp.Credentials{"token_v2": "tok", "base_url": ts.URL + "/"})
	require.NoError(t, err)
	assert.Equal(t, ts.URL, n.baseURL)
}

func TestConfigure_DefaultsToNotionBaseURL(t *testing.T) {
	n := New().(*notion)
	assert.Equal(t, "https://www.notion.so", n.baseURL)
}

func TestConfigure_ReturnsErrorWhenGetSpacesFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	n := &notion{client: ts.Client(), baseURL: ts.URL}
	err := n.Configure(context.Background(), mcp.Credentials{"token_v2": "bad-token"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve workspace")
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
	n := &notion{tokenV2: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := n.Execute(context.Background(), "notion_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

// --- HTTP helpers ---

func TestDoRequest_SetsCookieAuthHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := r.Header.Get("Cookie")
		assert.Contains(t, cookie, "token_v2=test-token")
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		// v3 has no Notion-Version header
		assert.Empty(t, r.Header.Get("Notion-Version"))
		okJSON(w, `{"id":"abc123"}`)
	}))
	defer ts.Close()

	n := &notion{tokenV2: "test-token", baseURL: ts.URL, client: ts.Client()}
	data, err := n.doRequest(context.Background(), "/api/v3/getRecordValues", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "abc123")
}

func TestDoRequest_ReturnsErrorOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	n := &notion{tokenV2: "bad-key", baseURL: ts.URL, client: ts.Client()}
	_, err := n.doRequest(context.Background(), "/api/v3/getSpaces", map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notion API error (401)")
}

func TestDoRequest_ExtractsMessageFromJSONError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"errorId":"abc-123","name":"ValidationError","message":"Invalid input: missing required field"}`))
	}))
	defer ts.Close()

	n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
	_, err := n.doRequest(context.Background(), "/api/v3/test", map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ValidationError")
	assert.Contains(t, err.Error(), "Invalid input: missing required field")
	// Should NOT contain the raw errorId noise
	assert.NotContains(t, err.Error(), "abc-123")
}

func TestDoRequest_TruncatesLongErrorBodies(t *testing.T) {
	longBody := `{"message":"` + string(make([]byte, 2000)) + `"}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(longBody))
	}))
	defer ts.Close()

	n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
	_, err := n.doRequest(context.Background(), "/api/v3/test", map[string]any{})
	require.Error(t, err)
	// Error should be truncated, not contain the full 2KB body
	assert.Less(t, len(err.Error()), 600)
}

func TestDoRequest_ReturnsRetryableErrorOn5xx(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"500", 500},
		{"502", 502},
		{"503", 503},
		{"504", 504},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"name":"ServerError","message":"internal"}`))
			}))
			defer ts.Close()

			n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
			_, err := n.doRequest(context.Background(), "/api/v3/test", map[string]any{})
			require.Error(t, err)
			assert.True(t, mcp.IsRetryable(err), "5xx should be retryable")
		})
	}
}

func TestDoRequest_ReturnsRetryableErrorOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"name":"RateLimited","message":"too many requests"}`))
	}))
	defer ts.Close()

	n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
	_, err := n.doRequest(context.Background(), "/api/v3/test", map[string]any{})
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "429 should be retryable")
}

func TestDoRequest_ParsesRetryAfterHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantDelay time.Duration
	}{
		{"parses seconds", "5", 5 * time.Second},
		{"caps at 60s", "120", 60 * time.Second},
		{"ignores invalid value", "not-a-number", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", tt.header)
				w.WriteHeader(429)
				_, _ = w.Write([]byte(`{"name":"RateLimited","message":"too many requests"}`))
			}))
			defer ts.Close()

			n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
			_, err := n.doRequest(context.Background(), "/api/v3/test", map[string]any{})
			require.Error(t, err)

			var re *mcp.RetryableError
			require.ErrorAs(t, err, &re)
			assert.Equal(t, tt.wantDelay, re.RetryAfter)
		})
	}
}

func TestDoRequest_ReturnsNonRetryableErrorOn4xx(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"400", 400},
		{"401", 401},
		{"403", 403},
		{"404", 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"name":"ClientError","message":"bad"}`))
			}))
			defer ts.Close()

			n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
			_, err := n.doRequest(context.Background(), "/api/v3/test", map[string]any{})
			require.Error(t, err)
			assert.False(t, mcp.IsRetryable(err), "4xx should NOT be retryable")
		})
	}
}

func TestDoRequest_ReturnsSuccessOn204(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := n.doRequest(context.Background(), "/api/v3/submitTransaction", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_PostsJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		okJSON(w, `{"ok":true}`)
	}))
	defer ts.Close()

	n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := n.doRequest(context.Background(), "/api/v3/submitTransaction", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "ok")
}

// --- Result helpers ---

func TestRawResult_WrapsDataWithoutError(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestJsonResult_MarshalsStructToJSON(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"hello": "world"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"hello":"world"`)
}

func TestErrResult_WrapsErrorMessage(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
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

func TestHealthy_ReturnsTrueWhenGetSpacesSucceeds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/getSpaces", r.URL.Path)
		okJSON(w, `{"user-1": {"space": {"sp-1": {"value": {}}}}}`)
	}))
	defer ts.Close()

	n := &notion{tokenV2: "key", baseURL: ts.URL, client: ts.Client()}
	assert.True(t, n.Healthy(context.Background()))
}

func TestHealthy_ReturnsFalseOnAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	n := &notion{tokenV2: "bad-key", baseURL: ts.URL, client: ts.Client()}
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
	return &notion{tokenV2: "test-token", spaceID: "space-1", userID: "user-1", baseURL: ts.URL, client: ts.Client()}
}

func TestTestNotion_SetsV3Fields(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{}`)
	})
	assert.Equal(t, "test-token", n.tokenV2)
	assert.Equal(t, "space-1", n.spaceID)
	assert.Equal(t, "user-1", n.userID)
}

func okJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// ===== Phase 2: Read handler tests =====
//
// v3 API pattern documentation (discovered via Playwright capture):
// - Block/page reads: loadCachedPageChunkV2 (not getRecordValues — shard isolation)
// - User reads: syncRecordValuesMain with pointer format (not getRecordValues)
// - Collection reads: loadCachedPageChunkV2 includes recordMap.collection
// - queryCollection: source + reducer format (not collection.id + loader.type: "table")

// --- loadBlock helper ---

func TestLoadBlock_FetchesViaPageChunk(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "blk-1", body["page"].(map[string]any)["id"])
		okJSON(w, `{
			"recordMap": {
				"block": {
					"blk-1": {"value": {"id": "blk-1", "type": "text", "properties": {"title": [["Hello"]]}}}
				}
			}
		}`)
	})
	result, err := loadBlock(context.Background(), n, "blk-1")
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "blk-1")
	assert.Contains(t, result.Data, "text")
}

// --- syncRecordValue helper ---

func TestSyncRecordValue_FetchesViaSyncMain(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/syncRecordValuesMain", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests := body["requests"].([]any)
		req := requests[0].(map[string]any)
		pointer := req["pointer"].(map[string]any)
		assert.Equal(t, "notion_user", pointer["table"])
		assert.Equal(t, "user-abc", pointer["id"])
		assert.Equal(t, float64(-1), req["version"])
		okJSON(w, `{
			"recordMap": {
				"notion_user": {
					"user-abc": {"value": {"id": "user-abc", "name": "Alice", "email": "alice@example.com"}}
				}
			}
		}`)
	})
	result, err := syncRecordValue(context.Background(), n, "notion_user", "user-abc")
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Alice")
}

// --- Single-record reads via loadCachedPageChunkV2 ---

func TestRetrieveBlock_ReturnsBlockFromRecordMap(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		okJSON(w, `{
			"recordMap": {
				"block": {
					"blk-1": {"value": {"id": "blk-1", "type": "text", "properties": {"title": [["Hello"]]}}}
				}
			}
		}`)
	})
	result, err := retrieveBlock(context.Background(), n, map[string]any{"block_id": "blk-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "blk-1")
	assert.Contains(t, result.Data, "text")
}

func TestRetrieveBlock_RequiresBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := retrieveBlock(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")
}

func TestRetrievePage_ReturnsPageFromRecordMap(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-abc": {"value": {"id": "page-abc", "type": "page", "properties": {"title": [["My Page"]]}}}
				}
			}
		}`)
	})
	result, err := retrievePage(context.Background(), n, map[string]any{"page_id": "page-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "page-abc")
}

func TestRetrieveDatabase_ReturnsCollectionFromRecordMap(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		// loadCachedPageChunkV2 on a collection_view_page block includes collection in recordMap
		okJSON(w, `{
			"recordMap": {
				"block": {
					"db-abc": {"value": {"id": "db-abc", "type": "collection_view_page", "collection_id": "col-1", "view_ids": ["v1"]}}
				},
				"collection": {
					"col-1": {"value": {"id": "col-1", "name": [["Tasks"]], "schema": {"col1": {"name": "Name", "type": "title"}}}}
				}
			}
		}`)
	})
	result, err := retrieveDatabase(context.Background(), n, map[string]any{"database_id": "db-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "col-1")
	assert.Contains(t, result.Data, "Tasks")
}

func TestRetrieveDatabase_RequiresDatabaseID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := retrieveDatabase(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "database_id is required")
}

func TestRetrieveDataSource_ReturnsCollectionFromRecordMap(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		// loadCachedPageChunkV2 on a collection_view_page includes collection in recordMap
		okJSON(w, `{
			"recordMap": {
				"block": {
					"ds-abc": {"value": {"id": "ds-abc", "type": "collection_view_page", "collection_id": "col-1", "view_ids": ["v1"]}}
				},
				"collection": {
					"col-1": {"value": {"id": "col-1", "name": [["Sprints"]], "schema": {}}}
				}
			}
		}`)
	})
	result, err := retrieveDataSource(context.Background(), n, map[string]any{"data_source_id": "ds-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "col-1")
	assert.Contains(t, result.Data, "Sprints")
}

func TestRetrieveUser_ReturnsUserFromRecordMap(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/syncRecordValuesMain", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		// syncRecordValuesMain uses pointer format (not flat table/id)
		requests := body["requests"].([]any)
		req := requests[0].(map[string]any)
		pointer := req["pointer"].(map[string]any)
		assert.Equal(t, "notion_user", pointer["table"])
		assert.Equal(t, "user-abc", pointer["id"])
		okJSON(w, `{
			"recordMap": {
				"notion_user": {
					"user-abc": {"value": {"id": "user-abc", "name": "Alice", "email": "alice@example.com"}}
				}
			}
		}`)
	})
	result, err := retrieveUser(context.Background(), n, map[string]any{"user_id": "user-abc"})
	require.NoError(t, err)
	assert.Contains(t, result.Data, "Alice")
}

func TestRetrieveUser_RequiresUserID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := retrieveUser(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "user_id is required")
}

func TestRetrievePageProperty_ReturnsPropertyFromPage(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "properties": {"title": [["My Page"]], "abc1": ["Done"]}}}
				}
			}
		}`)
	})
	result, err := retrievePageProperty(context.Background(), n, map[string]any{
		"page_id":     "page-1",
		"property_id": "title",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Page")
}

func TestRetrievePageProperty_RequiresPageIDAndPropertyID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })

	result, err := retrievePageProperty(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "page_id is required")

	result, err = retrievePageProperty(context.Background(), n, map[string]any{"page_id": "p1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "property_id is required")
}

// --- getPageContent via loadCachedPageChunkV2 ---

func TestGetPageContent_ReturnsPageAndBlocksFromChunkLoad(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "page-1", body["page"].(map[string]any)["id"])
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page", "content": ["blk-1", "blk-2"]}},
					"blk-1": {"value": {"id": "blk-1", "type": "text", "properties": {"title": [["Hello"]]}}},
					"blk-2": {"value": {"id": "blk-2", "type": "header", "properties": {"title": [["World"]]}}}
				}
			}
		}`)
	})
	result, err := getPageContent(context.Background(), n, map[string]any{"page_id": "page-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "page-1")
	assert.Contains(t, result.Data, "blk-1")
	assert.Contains(t, result.Data, "blk-2")
}

func TestGetPageContent_RequiresPageID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := getPageContent(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "page_id is required")
}

// --- getBlockChildren ---

func TestGetBlockChildren_ReturnsChildBlocksFromChunkLoad(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		// Single call: loadCachedPageChunkV2 returns parent + children
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path,
			"getBlockChildren should use loadCachedPageChunkV2 only (no getRecordValues)")
		okJSON(w, `{
			"recordMap": {
				"block": {
					"blk-parent": {"value": {"id": "blk-parent", "type": "page", "content": ["child-1", "child-2"]}},
					"child-1": {"value": {"id": "child-1", "type": "text", "parent_id": "blk-parent"}},
					"child-2": {"value": {"id": "child-2", "type": "header", "parent_id": "blk-parent"}},
					"unrelated": {"value": {"id": "unrelated", "type": "text", "parent_id": "other"}}
				}
			}
		}`)
	})
	result, err := getBlockChildren(context.Background(), n, map[string]any{"block_id": "blk-parent"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	results := out["results"].([]any)
	assert.Len(t, results, 2, "should return only direct children")
	assert.Contains(t, result.Data, "child-1")
	assert.Contains(t, result.Data, "child-2")
	assert.NotContains(t, result.Data, "unrelated")
}

func TestGetBlockChildren_RequiresBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := getBlockChildren(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")
}

// --- queryDataSource via queryCollection ---

func TestQueryDataSource_ReturnsResultsFromQueryCollection(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		switch r.URL.Path {
		case "/api/v3/loadCachedPageChunkV2":
			okJSON(w, `{
				"recordMap": {
					"block": {
						"cvp-1": {"value": {"id": "cvp-1", "type": "collection_view_page", "collection_id": "col-1", "view_ids": ["view-1"]}}
					}
				}
			}`)
		case "/api/v3/queryCollection":
			// v3 source+reducer format: source.id, loader.reducers
			source, _ := body["source"].(map[string]any)
			assert.Equal(t, "collection", source["type"], "source.type must be 'collection'")
			assert.Equal(t, "col-1", source["id"])
			loader, _ := body["loader"].(map[string]any)
			assert.NotNil(t, loader["reducers"], "loader must use reducers, not type: table")
			assert.NotNil(t, loader["sort"], "loader must include sort (even if empty)")

			// Response uses reducerResults, not flat blockIds
			okJSON(w, `{
				"result": {
					"type": "reducer",
					"reducerResults": {
						"collection_group_results": {
							"type": "results",
							"blockIds": ["row-1", "row-2"],
							"hasMore": false
						}
					},
					"sizeHint": 2
				},
				"allBlockIds": ["row-1", "row-2"],
				"recordMap": {
					"__version__": 3,
					"block": {
						"row-1": {"value": {"value": {"id": "row-1", "properties": {"title": [["Task 1"]]}}}},
						"row-2": {"value": {"value": {"id": "row-2", "properties": {"title": [["Task 2"]]}}}}
					}
				}
			}`)
		}
	})
	result, err := queryDataSource(context.Background(), n, map[string]any{"data_source_id": "cvp-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "row-1")
	assert.Contains(t, result.Data, "row-2")
}

func TestQueryDataSource_IncludesSchemaFromCollection(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/loadCachedPageChunkV2":
			okJSON(w, `{
				"recordMap": {
					"block": {
						"cvp-1": {"value": {"id": "cvp-1", "type": "collection_view_page", "collection_id": "col-1", "view_ids": ["view-1"]}}
					}
				}
			}`)
		case "/api/v3/queryCollection":
			okJSON(w, `{
				"result": {
					"type": "reducer",
					"reducerResults": {
						"collection_group_results": {
							"type": "results",
							"blockIds": ["row-1"],
							"hasMore": false
						}
					}
				},
				"recordMap": {
					"__version__": 3,
					"block": {
						"row-1": {"value": {"value": {"id": "row-1", "properties": {"title": [["Task 1"]], "gedz": [["Acme Corp"]]}}}}
					},
					"collection": {
						"col-1": {"value": {"value": {
							"id": "col-1",
							"name": [["Tasks"]],
							"schema": {
								"title": {"name": "Name", "type": "title"},
								"gedz": {"name": "Company", "type": "text"}
							}
						}}}
					}
				}
			}`)
		}
	})
	result, err := queryDataSource(context.Background(), n, map[string]any{"data_source_id": "cvp-1"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))

	schema, ok := resp["schema"].(map[string]any)
	require.True(t, ok, "response must include schema")
	gedz, ok := schema["gedz"].(map[string]any)
	require.True(t, ok, "schema must include gedz property")
	assert.Equal(t, "Company", gedz["name"])
	assert.Equal(t, "text", gedz["type"])
}

func TestQueryDataSource_RequiresDataSourceID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := queryDataSource(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "data_source_id is required")
}

// --- searchNotion via v3 search ---

func TestSearchNotion_NormalizesResultsWithRecordMapData(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/search", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "meeting notes", body["query"])
		assert.Equal(t, "space-1", body["spaceId"])
		okJSON(w, `{
			"results": [
				{"id": "page-1", "highlight": {"text": "Meeting Notes"}},
				{"id": "page-2", "highlight": {"text": "Project Plan"}}
			],
			"total": 2,
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page", "properties": {"title": [["Meeting Notes"]]}, "parent_id": "space-1", "created_time": 1700000000000, "last_edited_time": 1700000001000}},
					"page-2": {"value": {"id": "page-2", "type": "page", "properties": {"title": [["Project Plan"]]}, "parent_id": "space-1"}}
				}
			},
			"trackEventProperties": {"took": 123},
			"clusterInfo": {"isFullPageIndexing": true}
		}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{"query": "meeting notes"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	// Normalized: results merged with recordMap block data
	assert.Contains(t, result.Data, "page-1")
	assert.Contains(t, result.Data, "Meeting Notes")
	assert.Contains(t, result.Data, `"type":"page"`)
	assert.Contains(t, result.Data, `"total":2`)
	// Raw v3 noise should be stripped
	assert.NotContains(t, result.Data, "trackEventProperties")
	assert.NotContains(t, result.Data, "clusterInfo")
	assert.NotContains(t, result.Data, "recordMap")
}

func TestSearchNotion_FiltersResultsByType(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		// type must always be BlocksInSpace for the v3 API — never overwritten by user's filter
		assert.Equal(t, "BlocksInSpace", body["type"])
		okJSON(w, `{
			"results": [
				{"id": "page-1", "highlight": {"text": "a page"}},
				{"id": "db-1", "highlight": {"text": "a database"}}
			],
			"total": 2,
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page", "properties": {}}},
					"db-1": {"value": {"id": "db-1", "type": "collection_view_page", "properties": {}}}
				}
			}
		}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{"query": "test", "type": "page"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &out))
	results := out["results"].([]any)
	assert.Len(t, results, 1, "should filter to pages only")
	first := results[0].(map[string]any)
	assert.Equal(t, "page-1", first["id"])
}

func TestSearchNotion_CapsLimitAt100(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		limit, _ := body["limit"].(float64)
		assert.LessOrEqual(t, limit, float64(100), "limit should be capped at 100")
		okJSON(w, `{"results":[],"total":0,"recordMap":{}}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{"query": "test", "limit": 999})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryDataSource_CapsPageSizeAt100(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/api/v3/loadCachedPageChunkV2":
			okJSON(w, `{
				"recordMap": {
					"block": {
						"cvp-1": {"value": {"id": "cvp-1", "type": "collection_view_page", "collection_id": "col-1", "view_ids": ["view-1"]}}
					}
				}
			}`)
		case "/api/v3/queryCollection":
			// In source+reducer format, limit lives inside reducers.collection_group_results.limit
			loader, _ := body["loader"].(map[string]any)
			if loader != nil {
				reducers, _ := loader["reducers"].(map[string]any)
				if reducers != nil {
					cgr, _ := reducers["collection_group_results"].(map[string]any)
					if cgr != nil {
						limit, _ := cgr["limit"].(float64)
						assert.LessOrEqual(t, limit, float64(100), "page_size should be capped at 100")
					}
				}
			}
			okJSON(w, `{
				"result": {"type": "reducer", "reducerResults": {"collection_group_results": {"type": "results", "blockIds": [], "hasMore": false}}, "sizeHint": 0},
				"allBlockIds": [],
				"recordMap": {"block": {}}
			}`)
		}
	})
	result, err := queryDataSource(context.Background(), n, map[string]any{
		"data_source_id": "cvp-1",
		"page_size":      500,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- listUsers via getSpaces ---

func TestListUsers_ReturnsUsersFromGetSpaces(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/getSpaces", r.URL.Path)
		okJSON(w, `{
			"user-1": {
				"notion_user": {
					"u1": {"value": {"id": "u1", "name": "Alice", "email": "alice@test.com"}},
					"u2": {"value": {"id": "u2", "name": "Bob", "email": "bob@test.com"}}
				},
				"space": {"sp-1": {"value": {"id": "sp-1"}}}
			}
		}`)
	})
	result, err := listUsers(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Alice")
	assert.Contains(t, result.Data, "Bob")
}

// --- getSelf ---

func TestGetSelf_ReturnsStoredUserID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/syncRecordValuesMain", r.URL.Path)
		okJSON(w, `{
			"recordMap": {
				"notion_user": {
					"user-1": {"value": {"id": "user-1", "name": "Me", "email": "me@test.com"}}
				}
			}
		}`)
	})
	result, err := getSelf(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "user-1")
}

// --- retrieveComments via loadCachedPageChunkV2 ---

func TestRetrieveComments_ReturnsCommentsFromPageChunk(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page"}}
				},
				"discussion": {
					"disc-1": {"value": {"id": "disc-1", "parent_id": "page-1", "parent_table": "block", "comments": ["c1"]}}
				},
				"comment": {
					"c1": {"value": {"id": "c1", "parent_id": "disc-1", "text": [["Great work!"]], "created_by_id": "user-1", "created_time": 1700000000000}}
				}
			}
		}`)
	})
	result, err := retrieveComments(context.Background(), n, map[string]any{"block_id": "page-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Great work!")
	assert.Contains(t, result.Data, "disc-1")
}

func TestRetrieveComments_RequiresBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := retrieveComments(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")
}

func TestRetrieveComments_NoCommentsReturnsEmptyArray(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page"}}
				}
			}
		}`)
	})
	result, err := retrieveComments(context.Background(), n, map[string]any{"block_id": "page-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError, "pages with no comments should not error")
	assert.Contains(t, result.Data, `"results":[]`, "should return empty array, not null")
}

// --- listDataSourceTemplates ---

func TestListDataSourceTemplates_ReturnsTemplatesFromCollection(t *testing.T) {
	callCount := 0
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path,
			"listDataSourceTemplates must only use loadCachedPageChunkV2 (not getRecordValues)")

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		pageID := body["page"].(map[string]any)["id"].(string)

		switch pageID {
		case "cvp-1":
			// First call: resolve block + collection (includes template_pages)
			okJSON(w, `{
				"recordMap": {
					"block": {
						"cvp-1": {"value": {"id": "cvp-1", "type": "collection_view_page", "collection_id": "col-1", "view_ids": ["v1"]}}
					},
					"collection": {
						"col-1": {"value": {"id": "col-1", "template_pages": ["tmpl-1", "tmpl-2"]}}
					}
				}
			}`)
		case "tmpl-1":
			okJSON(w, `{
				"recordMap": {"block": {"tmpl-1": {"value": {"id": "tmpl-1", "type": "page", "properties": {"title": [["Bug Report"]]}}}}}
			}`)
		case "tmpl-2":
			okJSON(w, `{
				"recordMap": {"block": {"tmpl-2": {"value": {"id": "tmpl-2", "type": "page", "properties": {"title": [["Feature Request"]]}}}}}
			}`)
		}
	})
	result, err := listDataSourceTemplates(context.Background(), n, map[string]any{"data_source_id": "cvp-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "tmpl-1")
	assert.Contains(t, result.Data, "Bug Report")
}

func TestListDataSourceTemplates_RequiresDataSourceID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := listDataSourceTemplates(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "data_source_id is required")
}

// ===== Phase 3: Write handler tests =====

// --- createPage via submitTransaction ---

func TestCreatePage_SubmitsTransactionWithPageBlock(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Should have: set (block), update (block times), listAfter (parent content)
		require.GreaterOrEqual(t, len(body.Operations), 3)

		setOp := body.Operations[0]
		assert.Equal(t, "set", setOp.Command)
		assert.Equal(t, "block", setOp.Table)
		assert.NotEmpty(t, setOp.ID)

		// Verify the set op has page type and parent_id
		args, ok := setOp.Args.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "page", args["type"])
		assert.Equal(t, "parent-page-1", args["parent_id"])
		assert.Equal(t, "block", args["parent_table"])

		// Verify listAfter adds to parent's content
		listOp := body.Operations[len(body.Operations)-1]
		assert.Equal(t, "listAfter", listOp.Command)
		assert.Equal(t, "parent-page-1", listOp.ID)

		w.WriteHeader(200)
		okJSON(w, `{}`)
	})
	result, err := createPage(context.Background(), n, map[string]any{
		"parent": map[string]any{"page_id": "parent-page-1"},
		"title":  "New Page",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "id")
}

func TestCreatePage_ReturnsNotionURL(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := createPage(context.Background(), n, map[string]any{
		"parent": map[string]any{"page_id": "parent-page-1"},
		"title":  "Test Page",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	id, _ := resp["id"].(string)
	require.NotEmpty(t, id)

	url, _ := resp["url"].(string)
	dashless := strings.ReplaceAll(id, "-", "")
	assert.Equal(t, "https://www.notion.so/"+dashless, url)
}

func TestCreatePage_RequiresParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := createPage(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parent is required")
}

func TestCreatePage_SupportsParentDatabaseID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		setOp := body.Operations[0]
		args, _ := setOp.Args.(map[string]any)
		assert.Equal(t, "db-parent-1", args["parent_id"])
		assert.Equal(t, "collection", args["parent_table"])

		okJSON(w, `{}`)
	})
	result, err := createPage(context.Background(), n, map[string]any{
		"parent":     map[string]any{"database_id": "db-parent-1"},
		"properties": map[string]any{"title": [](any){[](any){"Task Name"}}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- updatePage via submitTransaction ---

func TestUpdatePage_SubmitsPropertyUpdates(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		require.GreaterOrEqual(t, len(body.Operations), 1)
		op := body.Operations[0]
		assert.Equal(t, "set", op.Command)
		assert.Equal(t, "block", op.Table)
		assert.Equal(t, "page-1", op.ID)
		assert.Equal(t, []string{"properties"}, stringSlice(op.Path))

		okJSON(w, `{}`)
	})
	result, err := updatePage(context.Background(), n, map[string]any{
		"page_id":    "page-1",
		"properties": map[string]any{"title": [](any){[](any){"Updated"}}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdatePage_RequiresPageID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := updatePage(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "page_id is required")
}

func TestUpdatePage_ArchivesPageWhenRequested(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Should have an operation setting alive to false
		found := false
		for _, op := range body.Operations {
			if op.Command == "set" && len(op.Path) > 0 && op.Path[0] == "alive" {
				found = true
				assert.Equal(t, false, op.Args)
			}
		}
		assert.True(t, found, "expected alive=false operation")

		okJSON(w, `{}`)
	})
	result, err := updatePage(context.Background(), n, map[string]any{
		"page_id":  "page-1",
		"archived": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- movePage via submitTransaction ---

func TestMovePage_SubmitsListRemoveAndListAfterOps(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/loadCachedPageChunkV2":
			// loadBlock: return current page with parent_id for the remove op
			okJSON(w, `{
				"recordMap": {
					"block": {
						"page-1": {"value": {"id": "page-1", "parent_id": "old-parent", "parent_table": "block"}}
					}
				}
			}`)
		case "/api/v3/submitTransaction":
			var body transaction
			_ = json.NewDecoder(r.Body).Decode(&body)

			commands := make([]string, len(body.Operations))
			for i, op := range body.Operations {
				commands[i] = op.Command
			}
			assert.Contains(t, commands, "listRemove")
			assert.Contains(t, commands, "listAfter")
			assert.Contains(t, commands, "set")

			okJSON(w, `{}`)
		}
	})
	result, err := movePage(context.Background(), n, map[string]any{
		"page_id": "page-1",
		"parent":  map[string]any{"page_id": "new-parent"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestMovePage_RequiresPageIDAndParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })

	result, err := movePage(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "page_id is required")

	result, err = movePage(context.Background(), n, map[string]any{"page_id": "p1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parent is required")
}

// --- createPageWithContent via submitTransaction ---

func TestCreatePageWithContent_SubmitsPageAndChildBlocksAtomically(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Should have ops for page + 2 child blocks + parent linkage
		// Minimum: page set + page update + parent listAfter + 2×(child set + child update + page listAfter)
		require.GreaterOrEqual(t, len(body.Operations), 6)

		// First set should be the page block
		pageSetOp := body.Operations[0]
		assert.Equal(t, "set", pageSetOp.Command)
		assert.Equal(t, "block", pageSetOp.Table)
		args, _ := pageSetOp.Args.(map[string]any)
		assert.Equal(t, "page", args["type"])

		okJSON(w, `{}`)
	})
	result, err := createPageWithContent(context.Background(), n, map[string]any{
		"parent": map[string]any{"page_id": "parent-1"},
		"title":  "My Page",
		"children": []any{
			map[string]any{"type": "text", "properties": map[string]any{"title": [](any){[](any){"First block"}}}},
			map[string]any{"type": "header", "properties": map[string]any{"title": [](any){[](any){"Second block"}}}},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "id")
}

func TestCreatePageWithContent_ReturnsNotionURL(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := createPageWithContent(context.Background(), n, map[string]any{
		"parent": map[string]any{"page_id": "parent-1"},
		"title":  "My Page",
		"children": []any{
			map[string]any{"type": "text", "properties": map[string]any{"title": [](any){[](any){"Block"}}}},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	id, _ := resp["id"].(string)
	require.NotEmpty(t, id)

	url, _ := resp["url"].(string)
	dashless := strings.ReplaceAll(id, "-", "")
	assert.Equal(t, "https://www.notion.so/"+dashless, url)
}

func TestCreatePageWithContent_RequiresParentAndChildren(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })

	result, err := createPageWithContent(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parent is required")

	result, err = createPageWithContent(context.Background(), n, map[string]any{
		"parent": map[string]any{"page_id": "p1"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "children is required")
}

// --- updateBlock via submitTransaction ---

func TestUpdateBlock_SubmitsSetOperationOnBlock(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		require.GreaterOrEqual(t, len(body.Operations), 1)
		op := body.Operations[0]
		assert.Equal(t, "set", op.Command)
		assert.Equal(t, "block", op.Table)
		assert.Equal(t, "blk-1", op.ID)
		assert.Equal(t, []string{"properties"}, stringSlice(op.Path))

		okJSON(w, `{}`)
	})
	result, err := updateBlock(context.Background(), n, map[string]any{
		"block_id":     "blk-1",
		"type_content": map[string]any{"properties": map[string]any{"title": [](any){[](any){"Updated text"}}}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateBlock_RequiresBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := updateBlock(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")
}

// --- deleteBlock via submitTransaction ---

func TestDeleteBlock_SubmitsAliveFalseAndListRemove(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/loadCachedPageChunkV2":
			// loadBlock: return block with parent info for listRemove
			okJSON(w, `{
				"recordMap": {
					"block": {
						"blk-1": {"value": {"id": "blk-1", "parent_id": "parent-1", "parent_table": "block"}}
					}
				}
			}`)
		case "/api/v3/submitTransaction":
			var body transaction
			_ = json.NewDecoder(r.Body).Decode(&body)

			commands := make([]string, len(body.Operations))
			for i, op := range body.Operations {
				commands[i] = op.Command
			}
			assert.Contains(t, commands, "set")
			assert.Contains(t, commands, "listRemove")

			for _, op := range body.Operations {
				if op.Command == "set" && len(op.Path) > 0 && op.Path[0] == "alive" {
					assert.Equal(t, false, op.Args)
				}
			}

			okJSON(w, `{}`)
		}
	})
	result, err := deleteBlock(context.Background(), n, map[string]any{"block_id": "blk-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteBlock_RequiresBlockID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := deleteBlock(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")
}

// --- appendBlockChildren via submitTransaction ---

func TestAppendBlockChildren_SubmitsSetAndListAfterPerChild(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		// 2 children: each needs set + update + listAfter = 6 ops minimum
		require.GreaterOrEqual(t, len(body.Operations), 6)

		// Count listAfter ops — should be at least 2 (one per child)
		listAfterCount := 0
		for _, op := range body.Operations {
			if op.Command == "listAfter" {
				listAfterCount++
				assert.Equal(t, "parent-blk", op.ID)
			}
		}
		assert.GreaterOrEqual(t, listAfterCount, 2)

		okJSON(w, `{}`)
	})
	result, err := appendBlockChildren(context.Background(), n, map[string]any{
		"block_id": "parent-blk",
		"children": []any{
			map[string]any{"type": "text", "properties": map[string]any{"title": [](any){[](any){"First"}}}},
			map[string]any{"type": "text", "properties": map[string]any{"title": [](any){[](any){"Second"}}}},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestAppendBlockChildren_RequiresBlockIDAndChildren(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })

	result, err := appendBlockChildren(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "block_id is required")

	result, err = appendBlockChildren(context.Background(), n, map[string]any{"block_id": "b1"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "children is required")
}

// --- createDatabase via submitTransaction ---

func TestCreateDatabase_SubmitsBlockCollectionAndViewOps(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Should create: collection_view_page block, collection, collection_view, + parent linkage
		tables := make(map[string]bool)
		for _, op := range body.Operations {
			tables[op.Table] = true
		}
		assert.True(t, tables["block"], "should have block operations")
		assert.True(t, tables["collection"], "should have collection operations")
		assert.True(t, tables["collection_view"], "should have collection_view operations")

		// Verify collection_view.parent_table = "block" (the gotcha)
		for _, op := range body.Operations {
			if op.Table == "collection_view" && op.Command == "set" {
				args, _ := op.Args.(map[string]any)
				if pt, ok := args["parent_table"]; ok {
					assert.Equal(t, "block", pt, "collection_view.parent_table must be 'block'")
				}
			}
		}

		okJSON(w, `{}`)
	})
	result, err := createDatabase(context.Background(), n, map[string]any{
		"parent":     map[string]any{"page_id": "parent-page"},
		"title":      "Tasks",
		"properties": map[string]any{"Name": map[string]any{"name": "Name", "type": "title"}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "id")
}

func TestCreateDatabase_ReturnsNotionURL(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := createDatabase(context.Background(), n, map[string]any{
		"parent":     map[string]any{"page_id": "parent-page"},
		"title":      "Tasks",
		"properties": map[string]any{"Name": map[string]any{"name": "Name", "type": "title"}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	id, _ := resp["id"].(string)
	require.NotEmpty(t, id)

	url, _ := resp["url"].(string)
	dashless := strings.ReplaceAll(id, "-", "")
	assert.Equal(t, "https://www.notion.so/"+dashless, url)
}

func TestCreateDatabase_RequiresParent(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := createDatabase(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "parent is required")
}

// --- updateDataSource via submitTransaction ---

func TestUpdateDataSource_SubmitsSchemaUpdate(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/loadCachedPageChunkV2":
			// resolveDataSource: resolve block ID → collection ID
			okJSON(w, `{
				"recordMap": {
					"block": {
						"ds-1": {"value": {"id": "ds-1", "type": "collection_view_page", "collection_id": "col-real", "view_ids": ["v1"]}}
					}
				}
			}`)
		case "/api/v3/submitTransaction":
			var body transaction
			_ = json.NewDecoder(r.Body).Decode(&body)

			require.GreaterOrEqual(t, len(body.Operations), 1)
			op := body.Operations[0]
			assert.Equal(t, "collection", op.Table)
			assert.Equal(t, "col-real", op.ID, "should use resolved collection ID, not block ID")

			okJSON(w, `{}`)
		}
	})
	result, err := updateDataSource(context.Background(), n, map[string]any{
		"data_source_id": "ds-1",
		"title":          "Updated Title",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateDataSource_RequiresDataSourceID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := updateDataSource(context.Background(), n, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "data_source_id is required")
}

// --- createComment via submitTransaction ---

func TestCreateComment_SubmitsDiscussionAndCommentOps(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Should create: discussion + comment + listAfter linkages
		tables := make(map[string]bool)
		for _, op := range body.Operations {
			tables[op.Table] = true
		}
		assert.True(t, tables["discussion"], "should have discussion operations")
		assert.True(t, tables["comment"], "should have comment operations")

		okJSON(w, `{}`)
	})
	result, err := createComment(context.Background(), n, map[string]any{
		"page_id": "page-1",
		"text":    "Great work!",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateComment_DiscussionHasResolvedFieldAndNoPrePopulatedComments(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		for _, op := range body.Operations {
			if op.Table == "discussion" && op.Command == "set" {
				args, ok := op.Args.(map[string]any)
				require.True(t, ok)

				// Must have resolved: false for Notion's NOT NULL constraint
				resolved, hasResolved := args["resolved"]
				assert.True(t, hasResolved, "discussion must include 'resolved' field")
				assert.Equal(t, false, resolved)

				// Should NOT pre-populate comments array (use listAfter instead)
				_, hasComments := args["comments"]
				assert.False(t, hasComments, "discussion should not pre-populate comments — use listAfter")
			}
		}
		okJSON(w, `{}`)
	})
	result, err := createComment(context.Background(), n, map[string]any{
		"page_id": "page-1",
		"text":    "test comment",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateComment_ReplyToExistingDiscussion(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body transaction
		_ = json.NewDecoder(r.Body).Decode(&body)

		// When replying, should NOT create a new discussion
		for _, op := range body.Operations {
			if op.Table == "discussion" && op.Command == "set" {
				t.Fatal("should not create new discussion when replying to existing one")
			}
		}

		// Should create comment + listAfter to discussion
		hasComment := false
		for _, op := range body.Operations {
			if op.Table == "comment" {
				hasComment = true
			}
		}
		assert.True(t, hasComment)

		okJSON(w, `{}`)
	})
	result, err := createComment(context.Background(), n, map[string]any{
		"discussion_id": "disc-1",
		"text":          "Reply text",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateComment_RequiresText(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := createComment(context.Background(), n, map[string]any{
		"page_id": "page-1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "text is required")
}

func TestCreateComment_RequiresPageIDOrDiscussionID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, _ *http.Request) { okJSON(w, `{}`) })
	result, err := createComment(context.Background(), n, map[string]any{
		"text": "Orphan comment",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "page_id or discussion_id is required")
}

// --- Test helper ---

// stringSlice converts []string from op.Path for assertion.
func stringSlice(path []string) []string {
	return path
}

// --- resolveDataSource: block ID → collection ID + view ID ---

func TestResolveDataSource_ResolvesBlockIDToCollectionAndViewIDs(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "cvp-block-1", body["page"].(map[string]any)["id"])
		okJSON(w, `{
			"recordMap": {
				"block": {
					"cvp-block-1": {"value": {"id": "cvp-block-1", "type": "collection_view_page", "collection_id": "col-real", "view_ids": ["view-1", "view-2"]}}
				},
				"collection": {
					"col-real": {"value": {"id": "col-real", "name": [["Tasks"]], "schema": {}}}
				}
			}
		}`)
	})
	colID, viewID, err := resolveDataSource(context.Background(), n, "cvp-block-1")
	require.NoError(t, err)
	assert.Equal(t, "col-real", colID)
	assert.Equal(t, "view-1", viewID)
}

func TestResolveDataSource_ErrorsWhenBlockHasNoCollectionID(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		okJSON(w, `{
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page"}}
				}
			}
		}`)
	})
	_, _, err := resolveDataSource(context.Background(), n, "page-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a database")
}

// --- queryDataSource resolves block IDs via loadCachedPageChunkV2 ---

func TestQueryDataSource_ResolvesBlockIDViaPageChunk(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		switch r.URL.Path {
		case "/api/v3/loadCachedPageChunkV2":
			okJSON(w, `{
				"recordMap": {
					"block": {
						"cvp-1": {"value": {"id": "cvp-1", "type": "collection_view_page", "collection_id": "col-real", "view_ids": ["view-1"]}}
					}
				}
			}`)
		case "/api/v3/queryCollection":
			// Verify source+reducer format with resolved IDs
			source := body["source"].(map[string]any)
			assert.Equal(t, "col-real", source["id"], "should use resolved collection ID in source")
			cv := body["collectionView"].(map[string]any)
			assert.Equal(t, "view-1", cv["id"], "should use resolved view ID")
			okJSON(w, `{
				"result": {"type": "reducer", "reducerResults": {"collection_group_results": {"type": "results", "blockIds": ["row-1"], "hasMore": false}}, "sizeHint": 1},
				"allBlockIds": ["row-1"],
				"recordMap": {"block": {"row-1": {"value": {"value": {"id": "row-1", "properties": {"title": [["Task 1"]]}}}}}}
			}`)
		}
	})
	result, err := queryDataSource(context.Background(), n, map[string]any{"data_source_id": "cvp-1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "row-1")
}

// --- retrieveDataSource/retrieveDatabase resolve block IDs ---

func TestRetrieveDataSource_ResolvesBlockIDToCollection(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/loadCachedPageChunkV2", r.URL.Path,
			"retrieveDataSource must only use loadCachedPageChunkV2")
		// Single call returns both block and collection in recordMap
		okJSON(w, `{
			"recordMap": {
				"block": {
					"cvp-block-1": {"value": {"id": "cvp-block-1", "type": "collection_view_page", "collection_id": "col-real", "view_ids": ["view-1"]}}
				},
				"collection": {
					"col-real": {"value": {"id": "col-real", "name": [["Tasks"]], "schema": {}}}
				}
			}
		}`)
	})
	result, err := retrieveDataSource(context.Background(), n, map[string]any{"data_source_id": "cvp-block-1"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "col-real")
	assert.Contains(t, result.Data, "Tasks")
}

// --- search includes collection_id for data source results ---

func TestSearchNotion_IncludesCollectionIDForDataSourceResults(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		okJSON(w, `{
			"results": [{"id": "cvp-1", "highlight": {"text": "My DB"}}],
			"total": 1,
			"recordMap": {
				"block": {
					"cvp-1": {"value": {"id": "cvp-1", "type": "collection_view_page", "collection_id": "col-abc", "view_ids": ["v1"]}}
				}
			}
		}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{"query": "My DB"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Contains(t, result.Data, "col-abc", "search results should include collection_id for data source blocks")
}

func TestSearchNotion_PassesThroughAllBlockTypes(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		okJSON(w, `{
			"results": [
				{"id": "page-1", "highlight": {"text": "Real Page"}},
				{"id": "ext-1", "highlight": {"text": "CRM Contact"}},
				{"id": "page-2", "highlight": {"text": "Another Page"}}
			],
			"total": 3,
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page", "properties": {"title": [["Real Page"]]}}},
					"ext-1": {"value": {"id": "ext-1", "type": "external_object_instance_page", "properties": {"title": [["CRM Contact"]]}}},
					"page-2": {"value": {"id": "page-2", "type": "page", "properties": {"title": [["Another Page"]]}}}
				}
			}
		}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{"query": "test"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Data), &resp))
	assert.Len(t, resp.Results, 3, "handler should pass through all block types — compaction handles noise")
}

func TestSearchNotion_StripsHighlightMarkup(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		okJSON(w, `{
			"results": [
				{"id": "page-1", "highlight": {"text": "Meeting <gzkNfoUU>Notes</gzkNfoUU> from Monday", "pathText": "Workspace / <gzkNfoUU>Team</gzkNfoUU>"}}
			],
			"total": 1,
			"recordMap": {
				"block": {
					"page-1": {"value": {"id": "page-1", "type": "page", "properties": {"title": [["Meeting Notes"]]}}}
				}
			}
		}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{"query": "notes"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	assert.NotContains(t, result.Data, "gzkNfoUU")
	assert.Contains(t, result.Data, "Meeting Notes from Monday")
	assert.Contains(t, result.Data, "Workspace / Team")
}

func TestSearchNotion_SynthesizesURL(t *testing.T) {
	n := testNotion(t, func(w http.ResponseWriter, r *http.Request) {
		okJSON(w, `{
			"results": [
				{"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890", "highlight": {"text": "Test"}}
			],
			"total": 1,
			"recordMap": {
				"block": {
					"a1b2c3d4-e5f6-7890-abcd-ef1234567890": {"value": {"id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890", "type": "page", "properties": {}}}
				}
			}
		}`)
	})
	result, err := searchNotion(context.Background(), n, map[string]any{"query": "test"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	assert.Contains(t, result.Data, "https://www.notion.so/a1b2c3d4e5f67890abcdef1234567890")
}

func TestStripHighlightTags(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"no tags", "no tags"},
		{"<gzkNfoUU>match</gzkNfoUU>", "match"},
		{"before <gzkNfoUU>match</gzkNfoUU> after", "before match after"},
		{"<abc>one</abc> and <def>two</def>", "one and two"},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, stripHighlightTags(tt.input), "input: %q", tt.input)
	}
}

// --- deterministic workspace selection ---

func TestResolveSpaceAndUser_DeterministicSelection(t *testing.T) {
	// When getSpaces returns multiple spaces, should deterministically select the first one
	// by sorting keys (not relying on Go map iteration order)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		okJSON(w, `{
			"user-zzz": {
				"space": {
					"space-bbb": {"value": {"id": "space-bbb"}},
					"space-aaa": {"value": {"id": "space-aaa"}}
				}
			},
			"user-aaa": {
				"space": {
					"space-ccc": {"value": {"id": "space-ccc"}}
				}
			}
		}`)
	}))
	defer ts.Close()

	// Run multiple times to verify determinism (Go map iteration is random)
	for i := 0; i < 10; i++ {
		n := &notion{client: ts.Client(), baseURL: ts.URL, tokenV2: "tok"}
		spaceID, userID, err := n.resolveSpaceAndUser(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "space-ccc", spaceID, "iteration %d: should consistently pick first sorted user's first sorted space", i)
		assert.Equal(t, "user-aaa", userID, "iteration %d: should consistently pick first sorted user", i)
	}
}

// Field compaction tests are in compact_specs_test.go
