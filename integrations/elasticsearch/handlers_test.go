package elasticsearch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) *esInt {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	e := &esInt{}
	err := e.Configure(context.Background(), mcp.Credentials{"base_url": ts.URL})
	require.NoError(t, err)
	return e
}

func TestSearch_RoundTrip(t *testing.T) {
	var gotPath, gotMethod, gotContentType string
	var gotBody map[string]any

	e := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[]}}`))
	})

	result, err := e.Execute(context.Background(), "elasticsearch_search", map[string]any{
		"index": "my-index",
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  float64(5),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.Equal(t, "/my-index/_search", gotPath)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, map[string]any{"match_all": map[string]any{}}, gotBody["query"])
	assert.Equal(t, float64(5), gotBody["size"])
}

func TestBulk_RoundTrip(t *testing.T) {
	var gotPath, gotContentType string
	var gotBody string

	e := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"took":30,"errors":false,"items":[]}`))
	})

	result, err := e.Execute(context.Background(), "elasticsearch_bulk", map[string]any{
		"operations": []any{
			map[string]any{"action": "index", "index": "test", "id": "1", "doc": map[string]any{"title": "hello"}},
			map[string]any{"action": "delete", "index": "test", "id": "2"},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.Equal(t, "/_bulk", gotPath)
	assert.Equal(t, "application/x-ndjson", gotContentType)
	assert.Contains(t, gotBody, `"index"`)
	assert.Contains(t, gotBody, `"delete"`)
}

func TestMsearch_RoundTrip(t *testing.T) {
	var gotPath, gotContentType string

	e := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"responses":[]}`))
	})

	result, err := e.Execute(context.Background(), "elasticsearch_msearch", map[string]any{
		"searches": []any{
			map[string]any{"index": "idx1", "body": map[string]any{"query": map[string]any{"match_all": map[string]any{}}}},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.Equal(t, "/_msearch", gotPath)
	assert.Equal(t, "application/x-ndjson", gotContentType)
}

func TestSQLQuery_InvalidFormat(t *testing.T) {
	e := setupTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})

	result, err := e.Execute(context.Background(), "elasticsearch_sql_query", map[string]any{
		"query":  "SELECT 1",
		"format": "xml",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid format")
}

func TestPathEscape_InPaths(t *testing.T) {
	var gotPath string

	e := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RawPath
		if gotPath == "" {
			gotPath = r.URL.Path
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"_index":"test","_id":"a/b","found":true,"_source":{}}`))
	})

	result, err := e.Execute(context.Background(), "elasticsearch_get_document", map[string]any{
		"index": "my-index",
		"id":    "a/b",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError, result.Data)
	assert.Contains(t, gotPath, "a%2Fb")
}

func TestRetryableError_On5xx(t *testing.T) {
	e := setupTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	})

	_, err := e.Execute(context.Background(), "elasticsearch_cluster_health", nil)
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err))
}
