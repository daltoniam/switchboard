package sentry

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
	assert.Equal(t, "sentry", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"auth_token": "sntrys_test", "organization": "my-org"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAuthToken(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"auth_token": "", "organization": "my-org"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth_token is required")
}

func TestConfigure_MissingOrganization(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/organizations/", r.URL.Path)
		w.Write([]byte(`[{"slug":"auto-org"}]`))
	}))
	defer ts.Close()

	s := &sentry{client: ts.Client(), baseURL: ts.URL}
	err := s.Configure(context.Background(), mcp.Credentials{"auth_token": "token", "organization": ""})
	assert.NoError(t, err)
	assert.Equal(t, "auto-org", s.organization)
}

func TestConfigure_AutoDetectOrgFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"detail":"unauthorized"}`))
	}))
	defer ts.Close()

	s := &sentry{client: ts.Client(), baseURL: ts.URL}
	err := s.Configure(context.Background(), mcp.Credentials{"auth_token": "bad-token", "organization": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auto-detect failed")
}

func TestConfigure_AutoDetectNoOrgs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	s := &sentry{client: ts.Client(), baseURL: ts.URL}
	err := s.Configure(context.Background(), mcp.Credentials{"auth_token": "token", "organization": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no organizations found")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	s := &sentry{client: &http.Client{}, baseURL: "https://sentry.io/api/0"}
	err := s.Configure(context.Background(), mcp.Credentials{
		"auth_token":   "token",
		"organization": "org",
		"base_url":     "https://custom.sentry.io/api/0/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.sentry.io/api/0", s.baseURL)
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

func TestTools_AllHaveSentryPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "sentry_", "tool %s missing sentry_ prefix", tool.Name)
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
	s := &sentry{authToken: "test", organization: "org", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "sentry_nonexistent", nil)
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
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	s := &sentry{authToken: "test-token", organization: "org", client: ts.Client(), baseURL: ts.URL}
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

	s := &sentry{authToken: "bad-token", organization: "org", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sentry API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &sentry{authToken: "token", organization: "org", client: ts.Client(), baseURL: ts.URL}
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

	s := &sentry{authToken: "token", organization: "org", client: ts.Client(), baseURL: ts.URL}
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

	s := &sentry{authToken: "token", organization: "org", client: ts.Client(), baseURL: ts.URL}
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

	s := &sentry{authToken: "token", organization: "org", client: ts.Client(), baseURL: ts.URL}
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

	s := &sentry{authToken: "token", organization: "org", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/test")
	require.Error(t, err)
	assert.False(t, mcp.IsRetryable(err), "404 should NOT be retryable")
}

// --- result helper tests ---

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

func TestErrResult_NonRetryable(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
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

func TestOrg(t *testing.T) {
	s := &sentry{organization: "default-org"}

	assert.Equal(t, "default-org", s.org(map[string]any{}))
	assert.Equal(t, "custom-org", s.org(map[string]any{"organization": "custom-org"}))
}
