package digitalocean

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/digitalocean/godo"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "digitalocean", i.Name())
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_WithToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"api_token": "dop_v1_test123",
	})
	assert.NoError(t, err)
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

func TestTools_AllHaveDOPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "digitalocean_", "tool %s missing digitalocean_ prefix", tool.Name)
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
	d := &integration{}
	err := d.Configure(context.Background(), mcp.Credentials{
		"api_token": "dop_v1_test123",
	})
	require.NoError(t, err)

	result, err := d.Execute(context.Background(), "digitalocean_nonexistent", nil)
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

func TestToolCount(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.Len(t, tools, len(dispatch), "tool count should match dispatch map size")
}

func TestJsonResult(t *testing.T) {
	result, err := mcp.JSONResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"key"`)
	assert.Contains(t, result.Data, `"value"`)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

func TestWrapRetryable_RateLimited(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests}
	godoErr := &godo.ErrorResponse{Response: resp}
	wrapped := wrapRetryable(godoErr)
	var re *mcp.RetryableError
	require.ErrorAs(t, wrapped, &re)
	assert.Equal(t, http.StatusTooManyRequests, re.StatusCode)
}

func TestWrapRetryable_ServerError(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusBadGateway}
	godoErr := &godo.ErrorResponse{Response: resp}
	wrapped := wrapRetryable(godoErr)
	var re *mcp.RetryableError
	require.ErrorAs(t, wrapped, &re)
	assert.Equal(t, http.StatusBadGateway, re.StatusCode)
}

func TestWrapRetryable_NonRetryable(t *testing.T) {
	err := fmt.Errorf("not found")
	assert.Equal(t, err, wrapRetryable(err))
}

func TestWrapRetryable_Nil(t *testing.T) {
	assert.Nil(t, wrapRetryable(nil))
}

// --- HTTP helper tests ---

func TestDoRequest_BearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	d := &integration{token: "test-token", httpClient: ts.Client(), baseURL: ts.URL}
	data, err := d.doGet(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer ts.Close()

	d := &integration{token: "bad", httpClient: ts.Client(), baseURL: ts.URL}
	_, err := d.doGet(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "digitalocean API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	d := &integration{token: "tok", httpClient: ts.Client(), baseURL: ts.URL}
	data, err := d.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	d := &integration{token: "tok", httpClient: ts.Client(), baseURL: ts.URL}
	data, err := d.doPost(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestDoRequest_Put(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Write([]byte(`{"updated":true}`))
	}))
	defer ts.Close()

	d := &integration{token: "tok", httpClient: ts.Client(), baseURL: ts.URL}
	data, err := d.doPut(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

func TestDoRequest_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	d := &integration{token: "tok", httpClient: ts.Client(), baseURL: ts.URL}
	data, err := d.doDel(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	d := &integration{token: "tok", httpClient: ts.Client(), baseURL: ts.URL}
	_, err := d.doGet(context.Background(), "/test")
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

	d := &integration{token: "tok", httpClient: ts.Client(), baseURL: ts.URL}
	_, err := d.doGet(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "503 should produce RetryableError")
}

func TestDoRequest_NonRetryableOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`not found`))
	}))
	defer ts.Close()

	d := &integration{token: "tok", httpClient: ts.Client(), baseURL: ts.URL}
	_, err := d.doGet(context.Background(), "/test")
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "404 should NOT be retryable")
}
