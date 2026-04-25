package signoz

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

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "signoz", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-key", "base_url": "https://signoz.example.com"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"base_url": "https://signoz.example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_MissingBaseURL(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-key"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base_url is required")
}

func TestConfigure_TrailingSlashTrimmed(t *testing.T) {
	s := &signoz{client: &http.Client{}}
	err := s.Configure(context.Background(), mcp.Credentials{"api_key": "key", "base_url": "https://example.com/"})
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com", s.baseURL)
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

func TestTools_AllHaveSignozPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, string(tool.Name), "signoz_", "tool %s missing signoz_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &signoz{apiKey: "test", baseURL: "http://localhost", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "signoz_nonexistent", nil)
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
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("SIGNOZ-API-KEY"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "test-key", baseURL: ts.URL, client: ts.Client()}
	data, err := s.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"detail":"forbidden"}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "bad-key", baseURL: ts.URL, client: ts.Client()}
	_, err := s.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signoz API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := s.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	data, err := s.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`{"detail":"rate limited"}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	_, err := s.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "429 should produce RetryableError")

	var re *mcp.RetryableError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, 429, re.StatusCode)
	assert.Equal(t, 30*time.Second, re.RetryAfter)
}

func TestDoRequest_RetryableOn5xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte(`service unavailable`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	_, err := s.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "503 should produce RetryableError")
}

func TestDoRequest_NonRetryableOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`not found`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	_, err := s.get(context.Background(), "/test")
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "404 should NOT be retryable")
}

// --- Result helper tests ---

func TestErrResult_PropagatesRetryableError(t *testing.T) {
	retryErr := &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
	result, err := mcp.ErrResult(retryErr)
	assert.Nil(t, result, "retryable error should not produce a ToolResult")
	assert.Error(t, err, "retryable error should be propagated as Go error")
	assert.True(t, mcp.IsRetryable(err))
}

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

// --- Healthy tests ---

func TestHealthy_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"version":"v0.120.0","ee":"Y","setupCompleted":true}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	assert.True(t, s.Healthy(context.Background()))
}

func TestHealthy_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`unauthorized`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "bad-key", baseURL: ts.URL, client: ts.Client()}
	assert.False(t, s.Healthy(context.Background()))
}

func TestHealthy_NilClient(t *testing.T) {
	s := &signoz{}
	assert.False(t, s.Healthy(context.Background()))
}

// --- Handler tests ---

func TestListDashboards(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/dashboards", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_list_dashboards", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "[]", result.Data)
}

func TestListAlerts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/rules", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Write([]byte(`{"status":"success","data":{"rules":[]}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_list_alerts", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "rules")
}

func TestGetVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/version", r.URL.Path)
		w.Write([]byte(`{"version":"v0.120.0","ee":"Y","setupCompleted":true}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_get_version", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "v0.120.0")
}

func TestSearchLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/query_range", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["compositeQuery"])
		w.Write([]byte(`{"status":"success","data":{"resultType":"","result":[{"queryName":"A"}]}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_search_logs", map[string]any{
		"start": "1700000000000",
		"end":   "1700003600000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "resultType")
}

func TestListServices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/services", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "1700000000000", body["start"])
		assert.Equal(t, "1700003600000", body["end"])
		w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_list_services", map[string]any{
		"start": "1700000000000",
		"end":   "1700003600000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "[]", result.Data)
}

func TestDeleteDashboard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/dashboards/abc-123", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_delete_dashboard", map[string]any{"id": "abc-123"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Compaction spec tests ---

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	toolNames := make(map[mcp.ToolName]bool)
	for name := range dispatch {
		toolNames[name] = true
	}
	for name := range rawFieldCompactionSpecs {
		assert.True(t, toolNames[name], "compaction spec %s has no dispatch handler", name)
	}
}

func TestFieldCompactionSpecs_AllReadToolsCovered(t *testing.T) {
	i := New()
	readPrefixes := []string{"signoz_list_", "signoz_get_", "signoz_search_", "signoz_query_", "signoz_top_", "signoz_entry_"}
	for _, tool := range i.Tools() {
		name := string(tool.Name)
		isRead := false
		for _, prefix := range readPrefixes {
			if strings.HasPrefix(name, prefix) {
				isRead = true
				break
			}
		}
		if !isRead {
			continue
		}
		_, ok := rawFieldCompactionSpecs[tool.Name]
		assert.True(t, ok, "read tool %s missing compaction spec", tool.Name)
	}
}

func TestParseFilterItems_Valid(t *testing.T) {
	f, err := parseFilterItems("severity_text = 'ERROR'", "")
	require.NoError(t, err)
	items := f["items"].([]any)
	assert.Len(t, items, 1)
}

func TestParseFilterItems_WithService(t *testing.T) {
	f, err := parseFilterItems("", "frontend")
	require.NoError(t, err)
	items := f["items"].([]any)
	assert.Len(t, items, 1)
}

func TestParseFilterItems_Empty(t *testing.T) {
	f, err := parseFilterItems("", "")
	require.NoError(t, err)
	items := f["items"].([]any)
	assert.Empty(t, items)
}

func TestParseFilterItems_Malformed(t *testing.T) {
	_, err := parseFilterItems("severity_text", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter format")
}

func TestSearchLogs_MalformedFilter(t *testing.T) {
	s := &signoz{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "signoz_search_logs", map[string]any{
		"start": "1700000000000", "end": "1700003600000", "filter": "bad",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid filter format")
}

func TestSearchLogs_InvalidTimestamp(t *testing.T) {
	s := &signoz{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "signoz_search_logs", map[string]any{
		"start": "not-a-number", "end": "1700003600000",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid epoch milliseconds")
}

func TestSearchTraces(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/query_range", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		cq := body["compositeQuery"].(map[string]any)
		bq := cq["builderQueries"].(map[string]any)
		a := bq["A"].(map[string]any)
		assert.Equal(t, "traces", a["dataSource"])
		w.Write([]byte(`{"status":"success","data":{"result":[{"queryName":"A"}]}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_search_traces", map[string]any{
		"start": "1700000000000", "end": "1700003600000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "result")
}

func TestGetTrace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/traces/abc123def", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Write([]byte(`[{"events":[]}]`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_get_trace", map[string]any{"trace_id": "abc123def"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/query_range", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		cq := body["compositeQuery"].(map[string]any)
		bq := cq["builderQueries"].(map[string]any)
		a := bq["A"].(map[string]any)
		assert.Equal(t, "metrics", a["dataSource"])
		assert.Equal(t, "sum", a["aggregateOperator"])
		w.Write([]byte(`{"status":"success","data":{"result":[{"queryName":"A","series":[]}]}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_query_metrics", map[string]any{
		"start": "1700000000000", "end": "1700003600000", "metric_name": "signoz_calls_total", "aggregate_op": "sum",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryMetrics_InvalidAggOp(t *testing.T) {
	s := &signoz{apiKey: "key", baseURL: "http://localhost", client: &http.Client{}}
	result, err := s.Execute(context.Background(), "signoz_query_metrics", map[string]any{
		"start": "1700000000000", "end": "1700003600000", "metric_name": "m", "aggregate_op": "bogus",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid aggregate_op")
}

func TestGetServiceOverview(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/query_range", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_get_service_overview", map[string]any{
		"service": "frontend", "start": "1700000000000", "end": "1700003600000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestTopOperations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/service/top_operations", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "frontend", body["service"])
		w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_top_operations", map[string]any{
		"service": "frontend", "start": "1700000000000", "end": "1700003600000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestTopLevelOperations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/service/top_level_operations", r.URL.Path)
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_top_level_operations", map[string]any{
		"start": "1700000000000", "end": "1700003600000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestEntryPointOperations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/service/entry_point_operations", r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "frontend", body["service"])
		w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_entry_point_operations", map[string]any{
		"service": "frontend", "start": "1700000000000", "end": "1700003600000",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetDashboard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/dashboards/dash-1", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Write([]byte(`{"status":"success","data":{"id":"dash-1","title":"My Dashboard"}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_get_dashboard", map[string]any{"id": "dash-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Dashboard")
}

func TestCreateDashboard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/dashboards", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Test Dashboard", body["title"])
		assert.Equal(t, "v5", body["version"])
		w.Write([]byte(`{"status":"success","data":{"id":"new-dash"}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_create_dashboard", map[string]any{"title": "Test Dashboard"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-dash")
}

func TestUpdateDashboard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/dashboards/dash-1", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Updated", body["title"])
		assert.Nil(t, body["id"], "id should be stripped — content only")
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_update_dashboard", map[string]any{
		"id": "dash-1", "dashboard": map[string]any{
			"id": "dash-1", "createdAt": "2024-01-01", "data": map[string]any{"title": "Updated", "widgets": []any{}},
		},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateDashboard_StringJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_update_dashboard", map[string]any{
		"id": "dash-1", "dashboard": `{"title":"From String"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetAlert(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/rules/42", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.Write([]byte(`{"status":"success","data":{"id":"42","alert":"cpu high"}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_get_alert", map[string]any{"id": "42"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "cpu high")
}

func TestCreateAlert(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/rules", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.Write([]byte(`{"status":"success","data":{"id":"99"}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_create_alert", map[string]any{
		"rule": map[string]any{"alert": "test-alert"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateAlert(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/rules/42", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_update_alert", map[string]any{
		"id": "42", "rule": map[string]any{"alert": "updated"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteAlert(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_delete_alert", map[string]any{"id": "42"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetSavedView(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/explorer/views/view-1", r.URL.Path)
		w.Write([]byte(`{"status":"success","data":{"uuid":"view-1","name":"My View"}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_get_saved_view", map[string]any{"view_id": "view-1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My View")
}

func TestCreateSavedView(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/explorer/views", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.Write([]byte(`{"status":"success","data":{"uuid":"new-view"}}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_create_saved_view", map[string]any{
		"view": map[string]any{"name": "Test View"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListChannels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/channels", r.URL.Path)
		w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_list_channels", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListSavedViews(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/explorer/views", r.URL.Path)
		w.Write([]byte(`{"status":"success","data":[]}`))
	}))
	defer ts.Close()

	s := &signoz{apiKey: "key", baseURL: ts.URL, client: ts.Client()}
	result, err := s.Execute(context.Background(), "signoz_list_saved_views", nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestExtractDashboardContent_FullGetResponse(t *testing.T) {
	obj := map[string]any{
		"id": "dash-1", "createdAt": "2024-01-01",
		"data": map[string]any{"title": "My Dashboard", "widgets": []any{}, "layout": []any{}},
	}
	content := extractDashboardContent(obj)
	assert.Equal(t, "My Dashboard", content["title"])
	assert.Nil(t, content["id"])
}

func TestExtractDashboardContent_AlreadyContent(t *testing.T) {
	obj := map[string]any{"title": "My Dashboard", "widgets": []any{}, "layout": []any{}}
	content := extractDashboardContent(obj)
	assert.Equal(t, "My Dashboard", content["title"])
}

func TestExtractDashboardContent_TripleNested(t *testing.T) {
	obj := map[string]any{
		"id": "dash-1",
		"data": map[string]any{
			"version": "v5",
			"data":    map[string]any{"title": "Deep", "widgets": []any{}},
		},
	}
	content := extractDashboardContent(obj)
	assert.Equal(t, "Deep", content["title"])
}

func TestExtractDashboardContent_DataWrapperNoID(t *testing.T) {
	obj := map[string]any{
		"data": map[string]any{"title": "Wrapped", "widgets": []any{}},
	}
	content := extractDashboardContent(obj)
	assert.Equal(t, "Wrapped", content["title"])
}
