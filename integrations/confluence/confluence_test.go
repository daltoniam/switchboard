package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "confluence", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"email":     "user@example.com",
		"api_token": "test-token",
		"domain":    "mycompany",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingEmail(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"api_token": "token",
		"domain":    "mycompany",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestConfigure_MissingAPIToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"email":  "user@example.com",
		"domain": "mycompany",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_token is required")
}

func TestConfigure_MissingDomain(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{
		"email":     "user@example.com",
		"api_token": "token",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain is required")
}

func TestConfigure_SetsURLs(t *testing.T) {
	c := &confluence{client: &http.Client{}}
	err := c.Configure(context.Background(), mcp.Credentials{
		"email":     "user@example.com",
		"api_token": "token",
		"domain":    "acme",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://acme.atlassian.net/wiki/api/v2", c.baseURL)
	assert.Equal(t, "https://acme.atlassian.net/wiki/rest/api", c.v1URL)
}

func TestPlainTextKeys(t *testing.T) {
	c := &confluence{}
	keys := c.PlainTextKeys()
	assert.Contains(t, keys, "email")
	assert.Contains(t, keys, "domain")
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

func TestTools_AllHaveConfluencePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "confluence_", "tool %s missing confluence_ prefix", tool.Name)
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
	c := &confluence{email: "test@test.com", apiToken: "test", domain: "test", client: &http.Client{}, baseURL: "http://localhost", v1URL: "http://localhost"}
	result, err := c.Execute(context.Background(), "confluence_nonexistent", nil)
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

// --- HTTP helper tests ---

func TestDoRequest_BasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		assert.Contains(t, auth, "Basic ")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	c := &confluence{email: "user@test.com", apiToken: "token", client: ts.Client(), baseURL: ts.URL, v1URL: ts.URL}
	data, err := c.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer ts.Close()

	c := &confluence{email: "user@test.com", apiToken: "bad", client: ts.Client(), baseURL: ts.URL, v1URL: ts.URL}
	_, err := c.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "confluence API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	c := &confluence{email: "user@test.com", apiToken: "token", client: ts.Client(), baseURL: ts.URL, v1URL: ts.URL}
	data, err := c.doRequest(context.Background(), "DELETE", ts.URL+"/test", nil)
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

	c := &confluence{email: "user@test.com", apiToken: "token", client: ts.Client(), baseURL: ts.URL, v1URL: ts.URL}
	data, err := c.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer ts.Close()

	c := &confluence{email: "user@test.com", apiToken: "token", client: ts.Client(), baseURL: ts.URL, v1URL: ts.URL}
	_, err := c.get(context.Background(), "/test")
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

	c := &confluence{email: "user@test.com", apiToken: "token", client: ts.Client(), baseURL: ts.URL, v1URL: ts.URL}
	_, err := c.get(context.Background(), "/test")
	require.Error(t, err)
	assert.True(t, mcp.IsRetryable(err), "503 should produce RetryableError")
}

func TestDoRequest_NonRetryableOn4xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`not found`))
	}))
	defer ts.Close()

	c := &confluence{email: "user@test.com", apiToken: "token", client: ts.Client(), baseURL: ts.URL, v1URL: ts.URL}
	_, err := c.get(context.Background(), "/test")
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "404 should NOT be retryable")
}

func TestV1Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()

	c := &confluence{email: "user@test.com", apiToken: "token", client: ts.Client(), baseURL: "http://other", v1URL: ts.URL}
	data, err := c.v1Get(context.Background(), "/search")
	require.NoError(t, err)
	assert.Contains(t, string(data), "results")
}

// --- result helper tests (shared mcp.RawResult / mcp.ErrResult) ---

func TestErrResult_PropagatesRetryableError(t *testing.T) {
	retryErr := &mcp.RetryableError{StatusCode: 503, Err: fmt.Errorf("service unavailable")}
	result, err := mcp.ErrResult(retryErr)
	assert.Nil(t, result, "retryable error should not produce a ToolResult")
	assert.Error(t, err, "retryable error should be propagated as Go error")
	assert.True(t, mcp.IsRetryable(err))
}

func TestErrResult_WrapsNonRetryableError(t *testing.T) {
	plainErr := fmt.Errorf("bad request")
	result, err := mcp.ErrResult(plainErr)
	require.NoError(t, err, "non-retryable error should not propagate as Go error")
	assert.True(t, result.IsError)
	assert.Equal(t, "bad request", result.Data)
}

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

// --- argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
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
