package posthog

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
	assert.Equal(t, "posthog", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"api_key": "phx_test123", "project_id": "12345"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"api_key": "", "project_id": "12345"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_MissingProjectID(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"api_key": "phx_test123", "project_id": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project_id is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	p := &posthog{client: &http.Client{}, baseURL: "https://us.posthog.com"}
	err := p.Configure(mcp.Credentials{
		"api_key":    "phx_test",
		"project_id": "1",
		"base_url":   "https://eu.posthog.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://eu.posthog.com", p.baseURL)
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

func TestTools_AllHavePosthogPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "posthog_", "tool %s missing posthog_ prefix", tool.Name)
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
	p := &posthog{apiKey: "test", projectID: "1", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := p.Execute(context.Background(), "posthog_nonexistent", nil)
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

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "test-token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	data, err := p.get(context.Background(), "/test")
	require.NoError(t, err)
	assert.Contains(t, string(data), "123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"detail":"forbidden"}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "bad-token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	_, err := p.get(context.Background(), "/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "posthog API error (403)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	data, err := p.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	data, err := p.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestPatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"updated":true}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	data, err := p.patch(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

// --- result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
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

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
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

func TestProj(t *testing.T) {
	p := &posthog{projectID: "default-proj"}

	assert.Equal(t, "default-proj", p.proj(map[string]any{}))
	assert.Equal(t, "custom-proj", p.proj(map[string]any{"project_id": "custom-proj"}))
}

// --- handler integration tests ---

func TestListFeatureFlags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/projects/1/feature_flags/")
		_, _ = w.Write([]byte(`{"results":[{"id":1,"key":"my-flag"}]}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_list_feature_flags", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my-flag")
}

func TestCreateFeatureFlag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/projects/1/feature_flags/")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "new-flag", body["key"])
		_, _ = w.Write([]byte(`{"id":2,"key":"new-flag"}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_create_feature_flag", map[string]any{
		"key": "new-flag",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new-flag")
}

func TestDeleteFeatureFlag_SoftDelete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, true, body["deleted"])
		_, _ = w.Write([]byte(`{"id":1,"deleted":true}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_delete_feature_flag", map[string]any{
		"flag_id": "1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deleted")
}

func TestListPersons(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/projects/1/persons/")
		_, _ = w.Write([]byte(`{"results":[{"id":"abc-123"}]}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_list_persons", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "abc-123")
}

func TestCreateDashboard(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "My Dashboard", body["name"])
		_, _ = w.Write([]byte(`{"id":5,"name":"My Dashboard"}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_create_dashboard", map[string]any{
		"name": "My Dashboard",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Dashboard")
}

func TestListExperiments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/projects/1/experiments/")
		_, _ = w.Write([]byte(`{"results":[{"id":1,"name":"Test AB"}]}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_list_experiments", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Test AB")
}

func TestCreateAnnotation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Deploy v2", body["content"])
		assert.Equal(t, "2024-01-15T00:00:00Z", body["date_marker"])
		_, _ = w.Write([]byte(`{"id":10,"content":"Deploy v2"}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_create_annotation", map[string]any{
		"content":     "Deploy v2",
		"date_marker": "2024-01-15T00:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Deploy v2")
}

func TestProjectOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/projects/99/")
		_, _ = w.Write([]byte(`{"id":"99"}`))
	}))
	defer ts.Close()

	p := &posthog{apiKey: "token", projectID: "1", client: ts.Client(), baseURL: ts.URL}
	result, err := p.Execute(context.Background(), "posthog_get_project", map[string]any{
		"project_id": "99",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
