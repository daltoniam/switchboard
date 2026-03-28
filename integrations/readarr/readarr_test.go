package readarr

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

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "readarr", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-key", "base_url": "http://localhost:8787"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"base_url": "http://localhost:8787"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_MissingBaseURL(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base_url is required")
}

func TestConfigure_TrimsTrailingSlash(t *testing.T) {
	ra := &readarr{client: &http.Client{}}
	err := ra.Configure(context.Background(), mcp.Credentials{
		"api_key":  "test",
		"base_url": "http://localhost:8787/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8787", ra.baseURL)
}

func TestPlainTextKeys(t *testing.T) {
	ra := &readarr{}
	keys := ra.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHavePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "readarr_", "tool %s missing readarr_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	ra := &readarr{apiKey: "test", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
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

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "field compaction spec %s has no dispatch handler", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("X-Api-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "test-key", client: ts.Client(), baseURL: ts.URL}
	data, err := ra.get(context.Background(), "/api/v1/system/status")
	require.NoError(t, err)
	assert.Contains(t, string(data), "1.0.0")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "bad-key", client: ts.Client(), baseURL: ts.URL}
	_, err := ra.get(context.Background(), "/api/v1/system/status")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "readarr API error (401)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	data, err := ra.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_RetryableError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	_, err := ra.get(context.Background(), "/api/v1/system/status")
	assert.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "RefreshBook", body["name"])
		_, _ = w.Write([]byte(`{"id":1,"name":"RefreshBook","status":"started"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	data, err := ra.post(context.Background(), "/api/v1/command", map[string]string{"name": "RefreshBook"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "RefreshBook")
}

func TestPut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	data, err := ra.put(context.Background(), "/api/v1/book/monitor", map[string]any{"bookIds": []int{1}, "monitored": true})
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

// --- Handler tests ---

func TestListBooks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/book")
		_, _ = w.Write([]byte(`[{"id":1,"title":"The Great Gatsby","authorTitle":"F. Scott Fitzgerald"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_books", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "The Great Gatsby")
}

func TestGetBook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/book/1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":1,"title":"The Great Gatsby"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_book", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "The Great Gatsby")
}

func TestGetBook_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_get_book", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id is required")
}

func TestSearchBooks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.RawQuery, "term=gatsby")
		_, _ = w.Write([]byte(`[{"book":{"title":"The Great Gatsby"}}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_search_books", map[string]any{"term": "gatsby"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "The Great Gatsby")
}

func TestSearchBooks_MissingTerm(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_search_books", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "term is required")
}

func TestMonitorBooks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/api/v1/book/monitor", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, true, body["monitored"])
		_, _ = w.Write([]byte(`[1, 2]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_monitor_books", map[string]any{
		"book_ids":  "1,2",
		"monitored": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestMonitorBooks_MissingIDs(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_monitor_books", map[string]any{"monitored": true})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "book_ids is required")
}

func TestListAuthors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/author", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"authorName":"F. Scott Fitzgerald"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_authors", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "F. Scott Fitzgerald")
}

func TestGetAuthor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/author/1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":1,"authorName":"F. Scott Fitzgerald","overview":"American novelist"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_author", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "F. Scott Fitzgerald")
}

func TestGetAuthor_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_get_author", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id is required")
}

func TestGetCalendar(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/calendar")
		_, _ = w.Write([]byte(`[{"id":1,"title":"New Release","releaseDate":"2024-06-01"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_calendar", map[string]any{
		"start": "2024-01-01",
		"end":   "2024-12-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "New Release")
}

func TestGetMissing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/wanted/missing")
		assert.Contains(t, r.URL.RawQuery, "page=1")
		_, _ = w.Write([]byte(`{"page":1,"pageSize":20,"totalRecords":1,"records":[{"id":1,"title":"Missing Book"}]}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_missing", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Missing Book")
}

func TestGetCutoff(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/wanted/cutoff")
		_, _ = w.Write([]byte(`{"page":1,"pageSize":20,"totalRecords":0,"records":[]}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_cutoff", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetQueue(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/queue")
		_, _ = w.Write([]byte(`{"page":1,"pageSize":20,"totalRecords":1,"records":[{"id":1,"title":"Downloading...","status":"downloading"}]}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_queue", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "downloading")
}

func TestDeleteQueueItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/queue/1")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_queue_item", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteQueueItem_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_delete_queue_item", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id is required")
}

func TestDeleteQueueBulk(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/queue/bulk")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		ids := body["ids"].([]any)
		assert.Len(t, ids, 2)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_queue_bulk", map[string]any{"ids": "1,2"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGrabQueueItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/queue/grab/1", r.URL.Path)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_grab_queue_item", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetHistory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/history")
		_, _ = w.Write([]byte(`{"page":1,"pageSize":20,"totalRecords":1,"records":[{"id":1,"eventType":"grabbed","date":"2024-01-01"}]}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_history", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "grabbed")
}

func TestGetHistoryAuthor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/history/author")
		assert.Contains(t, r.URL.RawQuery, "authorId=1")
		_, _ = w.Write([]byte(`[{"id":1,"eventType":"grabbed"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_history_author", map[string]any{"author_id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetHistoryAuthor_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_get_history_author", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "author_id is required")
}

func TestGetHistorySince(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/history/since")
		assert.Contains(t, r.URL.RawQuery, "date=2024-01-01")
		_, _ = w.Write([]byte(`[{"id":1,"eventType":"grabbed"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_history_since", map[string]any{"date": "2024-01-01"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetHistorySince_MissingDate(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_get_history_since", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "date is required")
}

func TestListCommands(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/command", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"name":"RefreshBook","status":"completed"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_commands", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "RefreshBook")
}

func TestRunCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/command", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "MissingBookSearch", body["name"])
		_, _ = w.Write([]byte(`{"id":1,"name":"MissingBookSearch","status":"started"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_run_command", map[string]any{"name": "MissingBookSearch"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "MissingBookSearch")
}

func TestRunCommand_MissingName(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_run_command", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "name is required")
}

func TestRunCommand_WithAuthorID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "AuthorSearch", body["name"])
		assert.Equal(t, float64(1), body["authorId"])
		_, _ = w.Write([]byte(`{"id":2,"name":"AuthorSearch","status":"started"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_run_command", map[string]any{
		"name":      "AuthorSearch",
		"author_id": float64(1),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/command/1", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":1,"name":"RefreshBook","status":"completed"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_command", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "completed")
}

func TestGetCommand_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_get_command", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id is required")
}

func TestGetSystemStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/system/status", r.URL.Path)
		_, _ = w.Write([]byte(`{"version":"0.3.0","osName":"linux"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_system_status", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "0.3.0")
}

func TestListRootFolders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/rootfolder", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"path":"/books","freeSpace":1073741824}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_root_folders", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "/books")
}

func TestListQualityProfiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/qualityprofile", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"name":"eBook"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_quality_profiles", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "eBook")
}

func TestListMetadataProfiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/metadataprofile", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"name":"Standard"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_metadata_profiles", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Standard")
}

func TestListTags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/tag", r.URL.Path)
		_, _ = w.Write([]byte(`[{"id":1,"label":"fiction"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_tags", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "fiction")
}

func TestHealthy_NotConfigured(t *testing.T) {
	ra := &readarr{client: &http.Client{}}
	assert.False(t, ra.Healthy(context.Background()))
}

func TestHealthy_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"0.3.0"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, ra.Healthy(context.Background()))
}

func TestCompactSpec(t *testing.T) {
	ra := &readarr{}
	fields, ok := ra.CompactSpec("readarr_list_books")
	assert.True(t, ok)
	assert.NotEmpty(t, fields)

	_, ok = ra.CompactSpec("readarr_nonexistent")
	assert.False(t, ok)
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- New tool handler tests ---

func TestAddBook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/book", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "12345", body["foreignBookId"])
		assert.Equal(t, true, body["monitored"])
		_, _ = w.Write([]byte(`{"id":1,"title":"New Book"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_add_book", map[string]any{
		"foreign_book_id":   "12345",
		"foreign_author_id": "67890",
		"root_folder_path":  "/books",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "New Book")
}

func TestAddBook_MissingFields(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_add_book", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "foreign_book_id is required")
}

func TestDeleteBook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/book/1")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_book", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteBook_WithOptions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "deleteFiles=true")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_book", map[string]any{
		"id": float64(1), "delete_files": true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestAddAuthor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/author", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "12345", body["foreignAuthorId"])
		assert.Equal(t, true, body["monitored"])
		_, _ = w.Write([]byte(`{"id":1,"authorName":"New Author"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_add_author", map[string]any{
		"foreign_author_id": "12345",
		"root_folder_path":  "/books",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "New Author")
}

func TestAddAuthor_MissingFields(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_add_author", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "foreign_author_id is required")
}

func TestUpdateAuthor(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == "GET" {
			_, _ = w.Write([]byte(`{"id":1,"authorName":"Author","monitored":true,"qualityProfileId":1}`))
			return
		}
		assert.Equal(t, "PUT", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, false, body["monitored"])
		_, _ = w.Write([]byte(`{"id":1,"authorName":"Author","monitored":false}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_update_author", map[string]any{
		"id": float64(1), "monitored": false,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "false")
}

func TestUpdateAuthor_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_update_author", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id is required")
}

func TestDeleteAuthor(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/author/1")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_author", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteAuthor_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_delete_author", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id is required")
}

func TestCreateTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/tag", r.URL.Path)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "scifi", body["label"])
		_, _ = w.Write([]byte(`{"id":1,"label":"scifi"}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_create_tag", map[string]any{"label": "scifi"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "scifi")
}

func TestCreateTag_MissingLabel(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_create_tag", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "label is required")
}

func TestDeleteTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/api/v1/tag/1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_tag", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListBookFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/bookfile")
		assert.Contains(t, r.URL.RawQuery, "authorId=1")
		_, _ = w.Write([]byte(`[{"id":1,"path":"/books/author/book.m4b","size":1024}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_book_files", map[string]any{"author_id": "1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "book.m4b")
}

func TestDeleteBookFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/api/v1/bookfile/1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_book_file", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListBlocklist(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/blocklist")
		_, _ = w.Write([]byte(`{"page":1,"pageSize":20,"totalRecords":1,"records":[{"id":1,"sourceTitle":"Bad Release"}]}`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_list_blocklist", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Bad Release")
}

func TestDeleteBlocklistItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/api/v1/blocklist/1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_delete_blocklist_item", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetRename(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/rename")
		assert.Contains(t, r.URL.RawQuery, "authorId=1")
		_, _ = w.Write([]byte(`[{"bookFileId":1,"existingPath":"/old.m4b","newPath":"/new.m4b"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_rename", map[string]any{"author_id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new.m4b")
}

func TestGetRename_MissingAuthorID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_get_rename", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "author_id is required")
}

func TestGetRetag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/retag")
		assert.Contains(t, r.URL.RawQuery, "authorId=1")
		_, _ = w.Write([]byte(`[{"bookFileId":1,"path":"/book.m4b","changes":[]}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_retag", map[string]any{"author_id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "book.m4b")
}

func TestGetManualImport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/manualimport")
		assert.Contains(t, r.URL.RawQuery, "folder=")
		_, _ = w.Write([]byte(`[{"id":1,"path":"/incoming/book.m4b","name":"book.m4b"}]`))
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_get_manual_import", map[string]any{"folder": "/incoming"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "book.m4b")
}

func TestGetManualImport_MissingFolder(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_get_manual_import", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "folder is required")
}

func TestMarkFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/history/failed/1", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ra := &readarr{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := ra.Execute(context.Background(), "readarr_mark_failed", map[string]any{"id": float64(1)})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestMarkFailed_MissingID(t *testing.T) {
	ra := &readarr{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ra.Execute(context.Background(), "readarr_mark_failed", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "id is required")
}
